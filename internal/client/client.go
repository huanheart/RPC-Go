package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kamaRPC/internal/limiter"
	"kamaRPC/internal/loadbalance"
	"kamaRPC/internal/protocol"
	"kamaRPC/internal/registry"
	"kamaRPC/internal/transport"
	"sync"
	"time"
)

// Client RPC 客户端
type Client struct {
	reg     *registry.Registry
	lb      loadbalance.LoadBalancer
	limiter *limiter.TokenBucket
	timeout time.Duration
	conns   sync.Map
}

// NewClient 创建新的客户端
func NewClient(reg *registry.Registry) *Client {
	return &Client{
		reg:     reg,
		lb:      &loadbalance.RoundRobin{},
		limiter: limiter.NewTokenBucket(100),
		timeout: 5 * time.Second,
	}
}

// Invoke 同步调用
func (c *Client) Invoke(ctx context.Context, service, method string, args interface{}, reply interface{}) error {
	// 检查限流
	if !c.limiter.Allow() {
		return errors.New("rate limit exceeded")
	}

	// 从负载均衡器获取地址
	addr, err := c.getAddr(service)
	if err != nil {
		return err
	}

	// 获取或创建连接
	conn := c.getConn(addr)
	if conn == nil {
		return errors.New("failed to connect")
	}
	defer conn.Close()

	// 构建请求
	body, _ := json.Marshal(args)
	req := &protocol.Message{
		Header: &protocol.Header{
			RequestID:   1,
			ServiceName: service,
			MethodName:  method,
		},
		Body: body,
	}

	// 发送请求
	if err := conn.WriteMessage(req); err != nil {
		return err
	}

	// 读取响应
	resp, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	// 检查错误
	if resp.Header.Error != "" {
		return errors.New(resp.Header.Error)
	}

	// 反序列化响应
	return json.Unmarshal(resp.Body, reply)
}

// getConn 获取或创建连接
func (c *Client) getConn(addr string) *transport.TCPClient {
	// 先检查缓存
	if conn, ok := c.conns.Load(addr); ok {
		tcpConn := conn.(*transport.TCPClient)
		if !tcpConn.IsClosed() {
			return tcpConn
		}
	}

	// 创建新连接
	conn, err := transport.NewTCPClient(addr)
	if err != nil {
		return nil
	}

	c.conns.Store(addr, conn)
	return conn
}

// getAddr 从注册中心和负载均衡器获取地址
func (c *Client) getAddr(service string) (string, error) {
	if c.reg == nil {
		return "", errors.New("registry not configured")
	}

	instances, err := c.reg.Discover(service)
	if err != nil {
		return "", err
	}

	if len(instances) == 0 {
		return "", errors.New("no instance available")
	}

	// 使用负载均衡器选择实例
	instance := c.lb.Select(instances)
	return instance.Addr, nil
}

// InvokeWithRetry 带重试的调用
func (c *Client) InvokeWithRetry(ctx context.Context, service, method string, args interface{}, reply interface{}, retries int) error {
	var lastErr error

	for i := 0; i <= retries; i++ {
		err := c.Invoke(ctx, service, method, args, reply)
		if err == nil {
			return nil
		}

		lastErr = err

		// 判断是否可重试
		if !isRetryable(err) {
			return err
		}

		// 指数退避
		if i < retries {
			backoff := time.Duration(1<<uint(i)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	return fmt.Errorf("after %d retries: %w", retries, lastErr)
}

// isRetryable 判断错误是否可重试
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	if errStr == "timeout" || errStr == "connection refused" || errStr == "broken pipe" {
		return true
	}

	if errStr == "service not found" || errStr == "rate limit exceeded" {
		return false
	}

	return true
}
