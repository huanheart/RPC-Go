package transport

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"kamaRPC/internal/protocol"
	"net"
	"sync"
	"time"
)

type TCPClient struct {
	conn net.Conn
	mu   sync.Mutex
	addr string
}

func newTCPClient(addr string) (*TCPClient, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &TCPClient{
		conn: conn,
		addr: addr,
	}, nil
}

func (c *TCPClient) WriteMessage(msg *protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := protocol.Encode(msg)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(data)
	return err
}

func (c *TCPClient) ReadMessage() (*protocol.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	reader := bufio.NewReaderSize(c.conn, 4096)

	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	fixedHeader := make([]byte, 10)
	if _, err := reader.Read(fixedHeader); err != nil {
		return nil, err
	}

	headerLen := binary.BigEndian.Uint32(fixedHeader[2:6])
	bodyLen := binary.BigEndian.Uint32(fixedHeader[6:10])

	headerBytes := make([]byte, headerLen)
	if _, err := reader.Read(headerBytes); err != nil {
		return nil, err
	}

	bodyBytes := make([]byte, bodyLen)
	if _, err := reader.Read(bodyBytes); err != nil {
		return nil, err
	}

	var header protocol.Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}

	return &protocol.Message{
		Header: &header,
		Body:   bodyBytes,
	}, nil
}

func (c *TCPClient) close() error {
	return c.conn.Close()
}

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
	if p.closed {
		return nil, ErrPoolClosed
	}

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
	if conn == nil || p.closed {
		return
	}

	select {
	case p.idleCh <- conn:
	default:
		conn.close()
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
		conn.close()
	}
}
