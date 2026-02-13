package transport

import (
	"errors"
	"kamaRPC/internal/protocol"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Call struct {
	resp *protocol.Message
	err  error
	done chan struct{}
}

type TCPClient struct {
	conn *TCPConnection
	addr string

	writeMu sync.Mutex

	// requestID 自增
	seq uint64

	// requestID -> call
	pending sync.Map // map[uint64]*Call

	closed int32
}

func newTCPClient(addr string) (*TCPClient, error) {
	rawConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	client := &TCPClient{
		conn: NewTCPConnection(rawConn),
		addr: addr,
	}

	go client.readLoop()

	return client, nil
}

func (c *TCPClient) nextSeq() uint64 {
	return atomic.AddUint64(&c.seq, 1)
}

func (c *TCPClient) SendRequest(msg *protocol.Message) (*protocol.Message, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errors.New("connection closed")
	}

	seq := c.nextSeq()
	msg.Header.RequestID = seq
	log.Println("当前序列号为: ", seq)

	call := &Call{
		done: make(chan struct{}),
	}

	c.pending.Store(seq, call)

	// 写必须是串行的(防止出现header header body body现象)
	c.writeMu.Lock()
	err := c.conn.Write(msg)
	c.writeMu.Unlock()

	if err != nil {
		c.pending.Delete(seq)
		return nil, err
	}

	// 等待响应
	<-call.done

	return call.resp, call.err
}

func (c *TCPClient) readLoop() {
	for {
		msg, err := c.conn.Read()
		if err != nil {
			c.closeAllPending(err)
			return
		}

		seq := msg.Header.RequestID

		value, ok := c.pending.Load(seq)
		if !ok {
			continue
		}

		call := value.(*Call)
		call.resp = msg
		call.done <- struct{}{}
		close(call.done)

		c.pending.Delete(seq)
	}
}

func (c *TCPClient) closeAllPending(err error) {
	c.pending.Range(func(key, value interface{}) bool {
		call := value.(*Call)
		call.err = err
		call.done <- struct{}{}
		close(call.done)
		return true
	})
}

func (c *TCPClient) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	return c.conn.Close()
}
