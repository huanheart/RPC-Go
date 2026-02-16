package client

import (
	"context"
	"errors"
	"kamaRPC/internal/codec"
	"kamaRPC/internal/limiter"
	"kamaRPC/internal/loadbalance"
	"kamaRPC/internal/protocol"
	"kamaRPC/internal/registry"
	"kamaRPC/internal/transport"
	"sync"
	"time"
)

type Client struct {
	reg     *registry.Registry
	lb      loadbalance.LoadBalancer
	limiter *limiter.TokenBucket
	timeout time.Duration
	codec   codec.Codec

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

func (c *Client) InvokeAsync(ctx context.Context, service string, method string, args interface{}) (*transport.Future, error) {

	if !c.limiter.Allow() {
		return nil, errors.New("rate limit exceeded")
	}

	addr, err := c.getAddr(service)
	if err != nil {
		return nil, err
	}

	pool := c.getPool(addr)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	body, err := c.codec.Marshal(args)
	if err != nil {
		return nil, err
	}

	req := &protocol.Message{
		Header: &protocol.Header{
			ServiceName: service,
			MethodName:  method,
			Compression: codec.CompressionGzip,
		},
		Body: body,
	}

	return conn.SendAsync(req)
}

// 同步接口 = 异步 + 等待
func (c *Client) Invoke(ctx context.Context, service string, method string, args interface{}, reply interface{}) error {

	future, err := c.InvokeAsync(ctx, service, method, args)
	if err != nil {
		return err
	}

	return future.GetResultWithContext(ctx, reply)
}

func (c *Client) getPool(addr string) *transport.ConnectionPool {
	if pool, ok := c.pools.Load(addr); ok {
		return pool.(*transport.ConnectionPool)
	}

	newPool := transport.NewConnectionPool(addr, 0, 1)
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

func (c *Client) Close() {
	c.pools.Range(func(key, value interface{}) bool {
		pool := value.(*transport.ConnectionPool)
		pool.Close()
		return true
	})
}
