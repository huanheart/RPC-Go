package transport

import (
	"bufio"
	"kamaRPC/internal/protocol"
	"net"
	"sync"
)

// BufferSize 定义缓冲区大小
const BufferSize = 4096

// PacketBuffer 用于粘包处理的缓冲区
type PacketBuffer struct {
	buf  []byte
	pos  int
	lock sync.Mutex
}

// Write 将数据写入缓冲区
func (pb *PacketBuffer) Write(data []byte) {
	pb.lock.Lock()
	defer pb.lock.Unlock()
	pb.buf = append(pb.buf, data...)
}

// Read 从缓冲区读取完整数据包，返回 nil 表示数据不完整
func (pb *PacketBuffer) Read() []byte {
	pb.lock.Lock()
	defer pb.lock.Unlock()

	// 检查是否至少有固定头部长度 (10 bytes: 2 magic + 4 headerLen + 4 bodyLen)
	if len(pb.buf) < 10 {
		return nil
	}

	// 解析 headerLen 和 bodyLen
	headerLen := int(protocol.DecodeHeaderLen(pb.buf[2:6]))
	bodyLen := int(protocol.DecodeBodyLen(pb.buf[6:10]))

	// 计算完整数据包长度
	totalLen := 10 + headerLen + bodyLen

	// 检查数据是否完整
	if len(pb.buf) < totalLen {
		return nil
	}

	// 提取完整数据包
	packet := make([]byte, totalLen)
	copy(packet, pb.buf[:totalLen])

	// 移动缓冲区数据
	copy(pb.buf, pb.buf[totalLen:])
	pb.buf = pb.buf[:len(pb.buf)-totalLen]

	return packet
}

// Len 返回当前缓冲区数据长度
func (pb *PacketBuffer) Len() int {
	pb.lock.Lock()
	defer pb.lock.Unlock()
	return len(pb.buf)
}

// TCPConnection 封装 TCP 连接
type TCPConnection struct {
	conn   net.Conn
	buffer *PacketBuffer
	reader *bufio.Reader
}

// NewTCPConnection 创建新的 TCP 连接封装
func NewTCPConnection(conn net.Conn) *TCPConnection {
	return &TCPConnection{
		conn:   conn,
		buffer: &PacketBuffer{buf: make([]byte, 0, BufferSize*2)},
		reader: bufio.NewReaderSize(conn, BufferSize),
	}
}

// Read 读取数据并处理粘包
func (tc *TCPConnection) Read() (*protocol.Message, error) {
	// 循环读取直到有完整数据包或出错
	for {
		// 先检查缓冲区是否有完整数据
		packet := tc.buffer.Read()
		if packet != nil {
			msg, err := protocol.DecodeBytes(packet)
			if err != nil {
				return nil, err
			}
			return msg, nil
		}

		// 尝试从连接读取数据
		data := make([]byte, BufferSize)
		n, err := tc.reader.Read(data)
		if err != nil {
			return nil, err
		}

		if n > 0 {
			// 将读取的数据写入缓冲区
			tc.buffer.Write(data[:n])
		}
	}
}

// Write 发送数据
func (tc *TCPConnection) Write(msg *protocol.Message) error {
	data, err := protocol.Encode(msg)
	if err != nil {
		return err
	}

	_, err = tc.conn.Write(data)
	return err
}

// Close 关闭连接
func (tc *TCPConnection) Close() error {
	return tc.conn.Close()
}

// RemoteAddr 获取远程地址
func (tc *TCPConnection) RemoteAddr() string {
	return tc.conn.RemoteAddr().String()
}

// TCPServer TCP 服务器
type TCPServer struct {
	listener  net.Listener
	connChan  chan *TCPConnection
	closeChan chan struct{}
	wg        sync.WaitGroup
}

// NewTCPServer 创建新的 TCP 服务器
func NewTCPServer(addr string) (*TCPServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &TCPServer{
		listener:  ln,
		connChan:  make(chan *TCPConnection, 100),
		closeChan: make(chan struct{}),
	}, nil
}

// Accept 接受连接并返回连接通道
func (ts *TCPServer) Accept() <-chan *TCPConnection {
	go func() {
		for {
			conn, err := ts.listener.Accept()
			if err != nil {
				select {
				case <-ts.closeChan:
					return
				default:
					continue
				}
			}

			tcpConn := NewTCPConnection(conn)
			select {
			case ts.connChan <- tcpConn:
			case <-ts.closeChan:
				conn.Close()
				return
			}
		}
	}()

	return ts.connChan
}

// Start 开始接受连接
func (ts *TCPServer) Start() {
	ts.Accept()
}

// Close 关闭服务器
func (ts *TCPServer) Close() {
	close(ts.closeChan)
	ts.listener.Close()
}

// Addr 获取监听地址
func (ts *TCPServer) Addr() string {
	return ts.listener.Addr().String()
}
