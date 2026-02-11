package transport

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"kamaRPC/internal/protocol"
	"net"
	"sync"
	"time"
)

// ConnectionPool 连接池
type ConnectionPool struct {
	addr        string
	mu          sync.Mutex
	connections []*TCPClient
	maxIdle     int
	maxActive   int
	idleChan    chan *TCPClient
	activeChan  chan struct{}
	wg          sync.WaitGroup
}

// NewConnectionPool 创建连接池
func NewConnectionPool(addr string, maxIdle, maxActive int) *ConnectionPool {
	pool := &ConnectionPool{
		addr:       addr,
		maxIdle:    maxIdle,
		maxActive:  maxActive,
		idleChan:   make(chan *TCPClient, maxIdle),
		activeChan: make(chan struct{}, maxActive),
	}
	return pool
}

// Get 获取连接
func (p *ConnectionPool) Get() (*TCPClient, error) {
	select {
	case conn := <-p.idleChan:
		return conn, nil
	default:
	}

	select {
	case p.activeChan <- struct{}{}:
	default:
		return nil, ErrPoolExhausted
	}

	conn, err := NewTCPClient(p.addr)
	if err != nil {
		<-p.activeChan
		return nil, err
	}

	return conn, nil
}

// Put 归还连接
func (p *ConnectionPool) Put(conn *TCPClient) {
	if conn == nil || conn.IsClosed() {
		<-p.activeChan
		return
	}

	select {
	case p.idleChan <- conn:
	default:
		conn.Close()
	}
}

// Close 关闭连接池
func (p *ConnectionPool) Close() {
	close(p.idleChan)
	close(p.activeChan)

	for conn := range p.idleChan {
		conn.Close()
	}
}

// ErrPoolExhausted 连接池耗尽
var ErrPoolExhausted = &poolError{"connection pool exhausted"}

type poolError struct {
	msg string
}

func (e *poolError) Error() string {
	return e.msg
}

// TCPClient TCP 客户端
type TCPClient struct {
	conn      net.Conn
	addr      string
	mu        sync.Mutex
	wg        sync.WaitGroup
	once      sync.Once
	closeChan chan struct{}
}

// NewTCPClient 创建新的 TCP 客户端
func NewTCPClient(addr string) (*TCPClient, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	client := &TCPClient{
		conn:      conn,
		addr:      addr,
		closeChan: make(chan struct{}),
	}

	return client, nil
}

// WriteMessage 发送消息
func (c *TCPClient) WriteMessage(msg *protocol.Message) error {
	data, err := protocol.Encode(msg)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(data)
	return err
}

// ReadMessage 读取消息
func (c *TCPClient) ReadMessage() (*protocol.Message, error) {
	reader := bufio.NewReaderSize(c.conn, 4096)

	// 设置读超时
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// 读取固定头部
	fixedHeader := make([]byte, 10)
	_, err := reader.Read(fixedHeader)
	if err != nil {
		return nil, err
	}

	// 解析 headerLen 和 bodyLen
	headerLen := binary.BigEndian.Uint32(fixedHeader[2:6])
	bodyLen := binary.BigEndian.Uint32(fixedHeader[6:10])

	// 读取完整头部
	headerBytes := make([]byte, headerLen)
	if _, err = reader.Read(headerBytes); err != nil {
		return nil, err
	}

	// 读取 body
	bodyBytes := make([]byte, bodyLen)
	if _, err = reader.Read(bodyBytes); err != nil {
		return nil, err
	}

	// 解析响应
	var header protocol.Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}

	return &protocol.Message{
		Header: &header,
		Body:   bodyBytes,
	}, nil
}

// Close 关闭客户端
func (c *TCPClient) Close() error {
	c.once.Do(func() {
		close(c.closeChan)
		c.wg.Wait()
	})
	return c.conn.Close()
}

// IsClosed 检查是否已关闭
func (c *TCPClient) IsClosed() bool {
	select {
	case <-c.closeChan:
		return true
	default:
		return false
	}
}

// Addr 获取地址
func (c *TCPClient) Addr() string {
	return c.addr
}
