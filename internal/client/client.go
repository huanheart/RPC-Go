package client

import (
	"kamaRPC/internal/breaker"
	"kamaRPC/internal/loadbalance"
	"kamaRPC/internal/registry"
)

type Client struct {
	reg     *registry.Registry
	lb      loadbalance.LoadBalancer
	breaker breaker.CircuitBreaker
	seq     uint64
}

func NewClient(reg *registry.Registry) *Client {
	return &Client{
		reg: reg,
		lb:  &loadbalance.RoundRobin{},
	}
}
