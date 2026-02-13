package client

import (
	"context"
	"errors"
	"fmt"
	"kamaRPC/internal/codec"
	"kamaRPC/internal/limiter"
	"kamaRPC/internal/loadbalance"
	"kamaRPC/internal/protocol"
	"kamaRPC/internal/registry"
	"kamaRPC/internal/transport"
	"sync"
	"time"
)

// ==============================
// Client RPC 客户端
// ==============================

type Client struct {
	reg     *registry.Registry
	lb      loadbalance.LoadBalancer
	limiter *limiter.TokenBucket
	timeout time.Duration
	codec   codec.Codec

	// 每个地址一个连接池
	pools sync.Map // map[string]*transport.ConnectionPool
}

func NewClient(reg *registry.Registry, opts ...ClientOption) (*Client, error) {
	c := &Client{
		reg:     reg,
		lb:      &loadbalance.RoundRobin{},
		limiter: limiter.NewTokenBucket(10000),
		timeout: 5 * time.Second,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}
func (c *Client) Invoke(ctx context.Context, service string, method string, args interface{}, reply interface{}) error {

	if !c.limiter.Allow() {
		return errors.New("rate limit exceeded")
	}

	addr, err := c.getAddr(service)
	if err != nil {
		return err
	}

	pool := c.getPool(addr)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}

	body, err := c.codec.Marshal(args)
	if err != nil {
		return err
	}

	req := &protocol.Message{
		Header: &protocol.Header{
			ServiceName: service,
			MethodName:  method,
		},
		Body: body,
	}

	resp, err := conn.SendRequest(req)
	// log.Println("resp is %v", resp.Header.RequestID)
	if err != nil {
		return err
	}

	if resp.Header.Error != "" {
		return errors.New(resp.Header.Error)
	}

	return c.codec.Unmarshal(resp.Body, reply)
}

func (c *Client) getPool(addr string) *transport.ConnectionPool {
	// 已存在
	if pool, ok := c.pools.Load(addr); ok {
		return pool.(*transport.ConnectionPool)
	}

	// 创建新的连接池
	newPool := transport.NewConnectionPool(
		addr,
		0, // maxIdle
		1, // maxActive
	)

	actual, _ := c.pools.LoadOrStore(addr, newPool)
	return actual.(*transport.ConnectionPool)
}

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

	instance := c.lb.Select(instances)
	return instance.Addr, nil
}

func (c *Client) InvokeWithRetry(ctx context.Context, service string, method string, args interface{}, reply interface{}, retries int) error {

	var lastErr error

	for i := 0; i <= retries; i++ {

		err := c.Invoke(ctx, service, method, args, reply)
		if err == nil {
			return nil
		}

		lastErr = err

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

func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	switch errStr {
	case "timeout",
		"connection refused",
		"broken pipe":
		return true

	case "service not found",
		"rate limit exceeded":
		return false
	}

	return true
}

func (c *Client) Close() {
	c.pools.Range(func(key, value interface{}) bool {
		pool := value.(*transport.ConnectionPool)
		pool.Close()
		return true
	})
}
