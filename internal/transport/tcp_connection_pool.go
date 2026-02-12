package transport

import (
	"context"
	"errors"
	"sync"
)

var ErrPoolClosed = errors.New("connection pool closed")
var ErrPoolExhausted = errors.New("connection pool exhausted")

type ConnectionPool struct {
	addr string

	maxIdle   int
	maxActive int

	idleCh   chan *TCPClient
	activeCh chan struct{}

	mu     sync.Mutex
	closed bool
}

func NewConnectionPool(addr string, maxIdle, maxActive int) *ConnectionPool {
	return &ConnectionPool{
		addr:      addr,
		maxIdle:   maxIdle,
		maxActive: maxActive,
		idleCh:    make(chan *TCPClient, maxIdle),
		activeCh:  make(chan struct{}, maxActive),
	}
}

func (p *ConnectionPool) Acquire(ctx context.Context) (*TCPClient, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	p.mu.Unlock()

	// 优先取 idle
	select {
	case conn := <-p.idleCh:
		return conn, nil
	default:
	}

	// 控制最大活跃连接
	select {
	case p.activeCh <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, ErrPoolExhausted
	}

	conn, err := newTCPClient(p.addr)
	if err != nil {
		<-p.activeCh
		return nil, err
	}

	return conn, nil
}

func (p *ConnectionPool) Release(conn *TCPClient) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		conn.Close()
		return
	}
	p.mu.Unlock()

	select {
	case p.idleCh <- conn:
	default:
		conn.Close()
		<-p.activeCh
	}
}

func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true

	close(p.idleCh)

	for conn := range p.idleCh {
		conn.Close()
	}
}
