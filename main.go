package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

///////////////////////
// Protocol Layer
///////////////////////

const (
	magic   uint16 = 0x1234
	version byte   = 1
)

type Header struct {
	RequestID   uint64
	ServiceName string
	MethodName  string
	Error       string
}

type Message struct {
	Header *Header
	Body   []byte
}

func encodeMessage(msg *Message) ([]byte, error) {
	headerBytes, err := json.Marshal(msg.Header)
	if err != nil {
		return nil, err
	}

	headerLen := uint32(len(headerBytes))
	bodyLen := uint32(len(msg.Body))

	totalLen := 2 + 1 + 4 + 4 + headerLen + bodyLen
	buf := make([]byte, totalLen)

	binary.BigEndian.PutUint16(buf[0:2], magic)
	buf[2] = version
	binary.BigEndian.PutUint32(buf[3:7], headerLen)
	binary.BigEndian.PutUint32(buf[7:11], bodyLen)

	copy(buf[11:11+headerLen], headerBytes)
	copy(buf[11+headerLen:], msg.Body)

	return buf, nil
}

func readMessage(conn net.Conn) (*Message, error) {
	fixedHeader := make([]byte, 11)
	if _, err := io.ReadFull(conn, fixedHeader); err != nil {
		return nil, err
	}

	if binary.BigEndian.Uint16(fixedHeader[0:2]) != magic {
		return nil, fmt.Errorf("invalid magic")
	}

	headerLen := binary.BigEndian.Uint32(fixedHeader[3:7])
	bodyLen := binary.BigEndian.Uint32(fixedHeader[7:11])

	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(conn, headerBytes); err != nil {
		return nil, err
	}

	bodyBytes := make([]byte, bodyLen)
	if _, err := io.ReadFull(conn, bodyBytes); err != nil {
		return nil, err
	}

	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}

	return &Message{
		Header: &header,
		Body:   bodyBytes,
	}, nil
}

///////////////////////
// Server
///////////////////////

type service struct {
	rcvr   reflect.Value
	method map[string]reflect.Method
}

type Server struct {
	services map[string]*service
}

func NewServer() *Server {
	return &Server{
		services: make(map[string]*service),
	}
}

func (s *Server) Register(name string, rcvr interface{}) {
	srv := &service{
		rcvr:   reflect.ValueOf(rcvr),
		method: make(map[string]reflect.Method),
	}

	typ := reflect.TypeOf(rcvr)
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		srv.method[m.Name] = m
	}

	s.services[name] = srv
}

func (s *Server) handle(conn net.Conn) {
	for {
		msg, err := readMessage(conn)
		if err != nil {
			return
		}

		go s.process(conn, msg)
	}
}

func (s *Server) process(conn net.Conn, msg *Message) {
	srv := s.services[msg.Header.ServiceName]
	method := srv.method[msg.Header.MethodName]

	argType := method.Type.In(1)
	replyType := method.Type.In(2)

	arg := reflect.New(argType.Elem())
	reply := reflect.New(replyType.Elem())

	json.Unmarshal(msg.Body, arg.Interface())

	method.Func.Call([]reflect.Value{
		srv.rcvr,
		arg,
		reply,
	})

	bodyBytes, _ := json.Marshal(reply.Interface())

	resp := &Message{
		Header: &Header{
			RequestID: msg.Header.RequestID,
		},
		Body: bodyBytes,
	}

	data, _ := encodeMessage(resp)
	conn.Write(data)
}

func (s *Server) Start(addr string) {
	ln, _ := net.Listen("tcp", addr)
	fmt.Println("Server listening on", addr)
	for {
		conn, _ := ln.Accept()
		go s.handle(conn)
	}
}

///////////////////////
// Client
///////////////////////

type Call struct {
	RequestID uint64
	Reply     interface{}
	Error     error
	Done      chan *Call
}

type Client struct {
	conn    net.Conn
	pending map[uint64]*Call
	mu      sync.Mutex
	seq     uint64
}

func NewClient(addr string) *Client {
	conn, _ := net.Dial("tcp", addr)
	c := &Client{
		conn:    conn,
		pending: make(map[uint64]*Call),
	}
	go c.receive()
	return c
}

func (c *Client) receive() {
	for {
		msg, err := readMessage(c.conn)
		if err != nil {
			return
		}

		c.mu.Lock()
		call := c.pending[msg.Header.RequestID]
		delete(c.pending, msg.Header.RequestID)
		c.mu.Unlock()

		if call != nil {
			json.Unmarshal(msg.Body, call.Reply)
			call.Done <- call
		}
	}
}

func (c *Client) Call(ctx context.Context, service, method string, args, reply interface{}) error {
	id := atomic.AddUint64(&c.seq, 1)

	bodyBytes, _ := json.Marshal(args)

	msg := &Message{
		Header: &Header{
			RequestID:   id,
			ServiceName: service,
			MethodName:  method,
		},
		Body: bodyBytes,
	}

	data, _ := encodeMessage(msg)

	call := &Call{
		RequestID: id,
		Reply:     reply,
		Done:      make(chan *Call, 1),
	}

	c.mu.Lock()
	c.pending[id] = call
	c.mu.Unlock()

	c.conn.Write(data)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case done := <-call.Done:
		return done.Error
	}
}

///////////////////////
// Demo Service
///////////////////////

type Args struct {
	A int
	B int
}

type Reply struct {
	Result int
}

type Arith struct{}

func (a *Arith) Add(args *Args, reply *Reply) {
	reply.Result = args.A + args.B
}

///////////////////////
// main
///////////////////////

func main() {

	go func() {
		server := NewServer()
		server.Register("Arith", &Arith{})
		server.Start(":8080")
	}()

	time.Sleep(time.Second)

	client := NewClient(":8080")

	for i := 0; i < 5; i++ {
		go func(i int) {
			args := &Args{A: i, B: i}
			reply := &Reply{}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			err := client.Call(ctx, "Arith", "Add", args, reply)
			if err != nil {
				fmt.Println("error:", err)
				return
			}

			fmt.Println("result:", reply.Result)
		}(i)
	}

	select {}
}
