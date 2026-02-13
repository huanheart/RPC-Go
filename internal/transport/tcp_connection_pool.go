package transport

import (
	"context"
	"errors"
	"sync"
)

var ErrPoolClosed = errors.New("connection pool closed")

type ConnectionPool struct {
	addr string

	maxActive int

	conns []*TCPClient
	mu    sync.Mutex

	closed bool
	next   int
}

func NewConnectionPool(addr string, maxIdle, maxActive int) *ConnectionPool {
	return &ConnectionPool{
		addr:      addr,
		maxActive: maxActive,
		conns:     make([]*TCPClient, 0, maxActive),
	}
}

func (p *ConnectionPool) Acquire(ctx context.Context) (*TCPClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrPoolClosed
	}

	// 未达到最大连接数 → 创建
	if len(p.conns) < p.maxActive {
		conn, err := newTCPClient(p.addr)
		if err != nil {
			return nil, err
		}
		p.conns = append(p.conns, conn)
		return conn, nil
	}

	// 轮询返回已有连接（共享）
	conn := p.conns[p.next]
	p.next = (p.next + 1) % len(p.conns)

	return conn, nil
}

func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true

	for _, conn := range p.conns {
		conn.Close()
	}
}
