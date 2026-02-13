// package transport

// import (
// 	"context"
// 	"errors"
// 	"sync"
// )

// var ErrPoolClosed = errors.New("connection pool closed")
// var ErrPoolExhausted = errors.New("connection pool exhausted")

// type ConnectionPool struct {
// 	addr string

// 	maxIdle   int
// 	maxActive int

// 	idleCh   chan *TCPClient
// 	activeCh chan struct{}

// 	mu     sync.Mutex
// 	closed bool
// }

// func NewConnectionPool(addr string, maxIdle, maxActive int) *ConnectionPool {
// 	return &ConnectionPool{
// 		addr:      addr,
// 		maxIdle:   maxIdle,
// 		maxActive: maxActive,
// 		idleCh:    make(chan *TCPClient, maxIdle),
// 		activeCh:  make(chan struct{}, maxActive),
// 	}
// }

// func (p *ConnectionPool) Acquire(ctx context.Context) (*TCPClient, error) {
// 	p.mu.Lock()
// 	if p.closed {
// 		p.mu.Unlock()
// 		return nil, ErrPoolClosed
// 	}
// 	p.mu.Unlock()

// 	// 优先取 idle
// 	select {
// 	case conn := <-p.idleCh:
// 		return conn, nil
// 	default:
// 	}

// 	// 控制最大活跃连接
// 	select {
// 	case p.activeCh <- struct{}{}:
// 	case <-ctx.Done():
// 		return nil, ctx.Err()
// 	default:
// 		return nil, ErrPoolExhausted
// 	}

// 	conn, err := newTCPClient(p.addr)
// 	if err != nil {
// 		<-p.activeCh
// 		return nil, err
// 	}

// 	return conn, nil
// }

// func (p *ConnectionPool) Release(conn *TCPClient) {
// 	if conn == nil {
// 		return
// 	}

// 	p.mu.Lock()
// 	if p.closed {
// 		p.mu.Unlock()
// 		conn.Close()
// 		return
// 	}
// 	p.mu.Unlock()

// 	select {
// 	case p.idleCh <- conn:
// 	default:
// 		conn.Close()
// 		<-p.activeCh
// 	}
// }

// func (p *ConnectionPool) Close() {
// 	p.mu.Lock()
// 	defer p.mu.Unlock()

// 	if p.closed {
// 		return
// 	}
// 	p.closed = true

// 	close(p.idleCh)

//		for conn := range p.idleCh {
//			conn.Close()
//		}
//	}
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
