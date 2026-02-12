package transport

import (
	"kamaRPC/internal/protocol"
	"net"
	"sync"
	"time"
)

type TCPClient struct {
	conn *TCPConnection
	mu   sync.Mutex
	addr string
}

func newTCPClient(addr string) (*TCPClient, error) {
	rawConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &TCPClient{
		conn: NewTCPConnection(rawConn),
		addr: addr,
	}, nil
}

func (c *TCPClient) WriteMessage(msg *protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Write(msg)
}

func (c *TCPClient) ReadMessage() (*protocol.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Read()
}

func (c *TCPClient) Close() error {
	return c.conn.Close()
}
