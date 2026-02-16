package transport

import (
	"errors"
	"kamaRPC/internal/protocol"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type TCPClient struct {
	conn *TCPConnection
	addr string

	writeMu sync.Mutex
	seq     uint64

	pending sync.Map // map[uint64]*Future

	closed int32
}

func newTCPClient(addr string) (*TCPClient, error) {
	rawConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	c := &TCPClient{
		conn: NewTCPConnection(rawConn),
		addr: addr,
	}

	go c.readLoop()
	return c, nil
}

func (c *TCPClient) nextSeq() uint64 {
	return atomic.AddUint64(&c.seq, 1)
}

func (c *TCPClient) SendAsync(msg *protocol.Message) (*Future, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errors.New("connection closed")
	}

	seq := c.nextSeq()
	msg.Header.RequestID = seq

	future := NewFuture()
	c.pending.Store(seq, future)

	c.writeMu.Lock()
	err := c.conn.Write(msg)
	c.writeMu.Unlock()

	if err != nil {
		c.pending.Delete(seq)
		return nil, err
	}

	return future, nil
}

func (c *TCPClient) readLoop() {
	for {
		msg, err := c.conn.Read()
		if err != nil {
			c.closeAllPending(err)
			return
		}

		seq := msg.Header.RequestID

		val, ok := c.pending.Load(seq)
		if !ok {
			continue
		}

		future := val.(*Future)

		if msg.Header.Error != "" {
			future.Done(nil, errors.New(msg.Header.Error))
		} else {
			future.Done(msg.Body, nil)
		}

		c.pending.Delete(seq)
	}
}

func (c *TCPClient) closeAllPending(err error) {
	c.pending.Range(func(key, value interface{}) bool {
		future := value.(*Future)
		future.Done(nil, err)
		return true
	})
}

func (c *TCPClient) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	return c.conn.Close()
}
