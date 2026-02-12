package transport

import (
	"bufio"
	"kamaRPC/internal/protocol"
	"net"
	"sync"
)

const BufferSize = 4096

type PacketBuffer struct {
	buf  []byte
	lock sync.Mutex
}

func (pb *PacketBuffer) Write(data []byte) {
	pb.lock.Lock()
	defer pb.lock.Unlock()
	pb.buf = append(pb.buf, data...)
}

func (pb *PacketBuffer) Read() []byte {
	pb.lock.Lock()
	defer pb.lock.Unlock()

	if len(pb.buf) < 10 {
		return nil
	}

	headerLen := int(protocol.DecodeHeaderLen(pb.buf[2:6]))
	bodyLen := int(protocol.DecodeBodyLen(pb.buf[6:10]))
	totalLen := 10 + headerLen + bodyLen

	if len(pb.buf) < totalLen {
		return nil
	}

	packet := make([]byte, totalLen)
	copy(packet, pb.buf[:totalLen])

	pb.buf = pb.buf[totalLen:]
	return packet
}

type TCPConnection struct {
	conn   net.Conn
	buffer *PacketBuffer
	reader *bufio.Reader
}

func NewTCPConnection(conn net.Conn) *TCPConnection {
	return &TCPConnection{
		conn:   conn,
		buffer: &PacketBuffer{buf: make([]byte, 0, BufferSize*2)},
		reader: bufio.NewReaderSize(conn, BufferSize),
	}
}

func (tc *TCPConnection) Read() (*protocol.Message, error) {
	for {
		packet := tc.buffer.Read()
		if packet != nil {
			return protocol.Decode(packet)
		}

		data := make([]byte, BufferSize)
		n, err := tc.reader.Read(data)
		if err != nil {
			return nil, err
		}

		if n > 0 {
			tc.buffer.Write(data[:n])
		}
	}
}

func (tc *TCPConnection) Write(msg *protocol.Message) error {
	data, err := protocol.Encode(msg)
	if err != nil {
		return err
	}

	_, err = tc.conn.Write(data)
	return err
}

func (tc *TCPConnection) Close() error {
	return tc.conn.Close()
}

func (tc *TCPConnection) RemoteAddr() string {
	return tc.conn.RemoteAddr().String()
}
