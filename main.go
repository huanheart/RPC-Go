// package main

// import (
// 	"context"
// 	"encoding/binary"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net"
// 	"reflect"
// 	"sync"
// 	"sync/atomic"
// 	"time"
// )

// ///////////////////////
// // Protocol Layer
// ///////////////////////

// const (
// 	magic   uint16 = 0x1234
// 	version byte   = 1
// )

// type Header struct {
// 	RequestID   uint64
// 	ServiceName string
// 	MethodName  string
// 	Error       string
// }

// type Message struct {
// 	Header *Header
// 	Body   []byte
// }

// func encodeMessage(msg *Message) ([]byte, error) {
// 	headerBytes, err := json.Marshal(msg.Header)
// 	if err != nil {
// 		return nil, err
// 	}

// 	headerLen := uint32(len(headerBytes))
// 	bodyLen := uint32(len(msg.Body))

// 	totalLen := 2 + 1 + 4 + 4 + headerLen + bodyLen
// 	buf := make([]byte, totalLen)

// 	binary.BigEndian.PutUint16(buf[0:2], magic)
// 	buf[2] = version
// 	binary.BigEndian.PutUint32(buf[3:7], headerLen)
// 	binary.BigEndian.PutUint32(buf[7:11], bodyLen)

// 	copy(buf[11:11+headerLen], headerBytes)
// 	copy(buf[11+headerLen:], msg.Body)

// 	return buf, nil
// }

// func readMessage(conn net.Conn) (*Message, error) {
// 	fixedHeader := make([]byte, 11)
// 	if _, err := io.ReadFull(conn, fixedHeader); err != nil {
// 		return nil, err
// 	}

// 	if binary.BigEndian.Uint16(fixedHeader[0:2]) != magic {
// 		return nil, fmt.Errorf("invalid magic")
// 	}

// 	headerLen := binary.BigEndian.Uint32(fixedHeader[3:7])
// 	bodyLen := binary.BigEndian.Uint32(fixedHeader[7:11])

// 	headerBytes := make([]byte, headerLen)
// 	if _, err := io.ReadFull(conn, headerBytes); err != nil {
// 		return nil, err
// 	}

// 	bodyBytes := make([]byte, bodyLen)
// 	if _, err := io.ReadFull(conn, bodyBytes); err != nil {
// 		return nil, err
// 	}

// 	var header Header
// 	if err := json.Unmarshal(headerBytes, &header); err != nil {
// 		return nil, err
// 	}

// 	return &Message{
// 		Header: &header,
// 		Body:   bodyBytes,
// 	}, nil
// }

// ///////////////////////
// // Server
// ///////////////////////

// type service struct {
// 	rcvr   reflect.Value
// 	method map[string]reflect.Method
// }

// type Server struct {
// 	services map[string]*service
// }

// func NewServer() *Server {
// 	return &Server{
// 		services: make(map[string]*service),
// 	}
// }

// func (s *Server) Register(name string, rcvr interface{}) {
// 	srv := &service{
// 		rcvr:   reflect.ValueOf(rcvr),
// 		method: make(map[string]reflect.Method),
// 	}

// 	typ := reflect.TypeOf(rcvr)
// 	for i := 0; i < typ.NumMethod(); i++ {
// 		m := typ.Method(i)
// 		srv.method[m.Name] = m
// 	}

// 	s.services[name] = srv
// }

// func (s *Server) handle(conn net.Conn) {
// 	for {
// 		msg, err := readMessage(conn)
// 		if err != nil {
// 			return
// 		}

// 		go s.process(conn, msg)
// 	}
// }

// func (s *Server) process(conn net.Conn, msg *Message) {
// 	srv := s.services[msg.Header.ServiceName]
// 	method := srv.method[msg.Header.MethodName]

// 	argType := method.Type.In(1)
// 	replyType := method.Type.In(2)

// 	arg := reflect.New(argType.Elem())
// 	reply := reflect.New(replyType.Elem())

// 	json.Unmarshal(msg.Body, arg.Interface())

// 	method.Func.Call([]reflect.Value{
// 		srv.rcvr,
// 		arg,
// 		reply,
// 	})

// 	bodyBytes, _ := json.Marshal(reply.Interface())

// 	resp := &Message{
// 		Header: &Header{
// 			RequestID: msg.Header.RequestID,
// 		},
// 		Body: bodyBytes,
// 	}

// 	data, _ := encodeMessage(resp)
// 	conn.Write(data)
// }

// func (s *Server) Start(addr string) {
// 	ln, _ := net.Listen("tcp", addr)
// 	fmt.Println("Server listening on", addr)
// 	for {
// 		conn, _ := ln.Accept()
// 		go s.handle(conn)
// 	}
// }

// ///////////////////////
// // Client
// ///////////////////////

// type Call struct {
// 	RequestID uint64
// 	Reply     interface{}
// 	Error     error
// 	Done      chan *Call
// }

// type Client struct {
// 	conn    net.Conn
// 	pending map[uint64]*Call
// 	mu      sync.Mutex
// 	seq     uint64
// }

// func NewClient(addr string) *Client {
// 	conn, _ := net.Dial("tcp", addr)
// 	c := &Client{
// 		conn:    conn,
// 		pending: make(map[uint64]*Call),
// 	}
// 	go c.receive()
// 	return c
// }

// func (c *Client) receive() {
// 	for {
// 		msg, err := readMessage(c.conn)
// 		if err != nil {
// 			return
// 		}

// 		c.mu.Lock()
// 		call := c.pending[msg.Header.RequestID]
// 		delete(c.pending, msg.Header.RequestID)
// 		c.mu.Unlock()

// 		if call != nil {
// 			json.Unmarshal(msg.Body, call.Reply)
// 			call.Done <- call
// 		}
// 	}
// }

// func (c *Client) Call(ctx context.Context, service, method string, args, reply interface{}) error {
// 	id := atomic.AddUint64(&c.seq, 1)

// 	bodyBytes, _ := json.Marshal(args)

// 	msg := &Message{
// 		Header: &Header{
// 			RequestID:   id,
// 			ServiceName: service,
// 			MethodName:  method,
// 		},
// 		Body: bodyBytes,
// 	}

// 	data, _ := encodeMessage(msg)

// 	call := &Call{
// 		RequestID: id,
// 		Reply:     reply,
// 		Done:      make(chan *Call, 1),
// 	}

// 	c.mu.Lock()
// 	c.pending[id] = call
// 	c.mu.Unlock()

// 	c.conn.Write(data)

// 	select {
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	case done := <-call.Done:
// 		return done.Error
// 	}
// }

// ///////////////////////
// // Demo Service
// ///////////////////////

// type Args struct {
// 	A int
// 	B int
// }

// type Reply struct {
// 	Result int
// }

// type Arith struct{}

// func (a *Arith) Add(args *Args, reply *Reply) {
// 	reply.Result = args.A + args.B
// }

// ///////////////////////
// // main
// ///////////////////////

// func main() {

// 	go func() {
// 		server := NewServer()
// 		server.Register("Arith", &Arith{})
// 		server.Start(":8080")
// 	}()

// 	time.Sleep(time.Second)

// 	client := NewClient(":8080")

// 	for i := 0; i < 5; i++ {
// 		go func(i int) {
// 			args := &Args{A: i, B: i}
// 			reply := &Reply{}

// 			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
// 			defer cancel()

// 			err := client.Call(ctx, "Arith", "Add", args, reply)
// 			if err != nil {
// 				fmt.Println("error:", err)
// 				return
// 			}

// 			fmt.Println("result:", reply.Result)
// 		}(i)
// 	}

//		select {}
//	}
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

/////////////////////////////////////////////////////
// ================= PROTOCOL ======================
/////////////////////////////////////////////////////

const magic uint16 = 0x1234

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

func encode(msg *Message) ([]byte, error) {
	hb, _ := json.Marshal(msg.Header)
	headerLen := uint32(len(hb))
	bodyLen := uint32(len(msg.Body))

	total := 2 + 4 + 4 + headerLen + bodyLen
	buf := make([]byte, total)

	binary.BigEndian.PutUint16(buf[0:2], magic)
	binary.BigEndian.PutUint32(buf[2:6], headerLen)
	binary.BigEndian.PutUint32(buf[6:10], bodyLen)

	copy(buf[10:], hb)
	copy(buf[10+headerLen:], msg.Body)
	return buf, nil
}

func readMsg(conn net.Conn) (*Message, error) {
	fixed := make([]byte, 10)
	_, err := io.ReadFull(conn, fixed)
	if err != nil {
		return nil, err
	}
	if binary.BigEndian.Uint16(fixed[0:2]) != magic {
		return nil, fmt.Errorf("invalid magic")
	}

	headerLen := binary.BigEndian.Uint32(fixed[2:6])
	bodyLen := binary.BigEndian.Uint32(fixed[6:10])

	hb := make([]byte, headerLen)
	io.ReadFull(conn, hb)

	bb := make([]byte, bodyLen)
	io.ReadFull(conn, bb)

	var header Header
	json.Unmarshal(hb, &header)

	return &Message{
		Header: &header,
		Body:   bb,
	}, nil
}

/////////////////////////////////////////////////////
// ================= REGISTRY ======================
/////////////////////////////////////////////////////

type Instance struct {
	Addr string
}

type Registry struct {
	mu       sync.RWMutex
	services map[string][]Instance
	watchers map[string][]chan []Instance
}

func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string][]Instance),
		watchers: make(map[string][]chan []Instance),
	}
}

func (r *Registry) Register(service string, ins Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.services[service] = append(r.services[service], ins)

	for _, ch := range r.watchers[service] {
		ch <- r.services[service]
	}
}

func (r *Registry) Watch(service string) chan []Instance {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan []Instance, 1)
	r.watchers[service] = append(r.watchers[service], ch)
	return ch
}

func (r *Registry) Get(service string) []Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.services[service]
}

/////////////////////////////////////////////////////
// ================= LOAD BALANCE ==================
/////////////////////////////////////////////////////

type LoadBalancer interface {
	Select([]Instance) Instance
}

type RoundRobin struct {
	idx uint64
}

func (r *RoundRobin) Select(list []Instance) Instance {
	i := atomic.AddUint64(&r.idx, 1)
	return list[i%uint64(len(list))]
}

/////////////////////////////////////////////////////
// ================= RETRY =========================
/////////////////////////////////////////////////////

func retry(attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		backoff := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
		jitter := time.Duration(rand.Intn(100)) * time.Millisecond
		time.Sleep(backoff + jitter)
	}
	return err
}

/////////////////////////////////////////////////////
// ================= CIRCUIT BREAKER ==============
/////////////////////////////////////////////////////

type CircuitBreaker struct {
	failures int
	state    int // 0 closed, 1 open
	mu       sync.Mutex
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == 0
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	if cb.failures > 3 {
		cb.state = 1
		go func() {
			time.Sleep(2 * time.Second)
			cb.mu.Lock()
			cb.state = 0
			cb.failures = 0
			cb.mu.Unlock()
		}()
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	cb.failures = 0
	cb.mu.Unlock()
}

/////////////////////////////////////////////////////
// ================= LIMITER =======================
/////////////////////////////////////////////////////

type TokenBucket struct {
	tokens int
	rate   int
	mu     sync.Mutex
}

func NewTokenBucket(rate int) *TokenBucket {
	tb := &TokenBucket{tokens: rate, rate: rate}
	go func() {
		for {
			time.Sleep(time.Second)
			tb.mu.Lock()
			tb.tokens = tb.rate
			tb.mu.Unlock()
		}
	}()
	return tb
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

/////////////////////////////////////////////////////
// ================= INTERCEPTOR ===================
/////////////////////////////////////////////////////

type Handler func(ctx context.Context, req interface{}) (interface{}, error)

type Interceptor func(ctx context.Context, req interface{}, next Handler) (interface{}, error)

func Chain(interceptors ...Interceptor) Interceptor {
	return func(ctx context.Context, req interface{}, final Handler) (interface{}, error) {
		var build func(int, context.Context, interface{}) (interface{}, error)
		build = func(i int, ctx context.Context, req interface{}) (interface{}, error) {
			if i == len(interceptors) {
				return final(ctx, req)
			}
			return interceptors[i](ctx, req, func(c context.Context, r interface{}) (interface{}, error) {
				return build(i+1, c, r)
			})
		}
		return build(0, ctx, req)
	}
}

/////////////////////////////////////////////////////
// ================= SERVER ========================
/////////////////////////////////////////////////////

type Server struct {
	addr    string
	srv     interface{}
	limiter *TokenBucket
}

func NewServer(addr string, srv interface{}) *Server {
	return &Server{
		addr:    addr,
		srv:     srv,
		limiter: NewTokenBucket(5),
	}
}

func (s *Server) Start(reg *Registry, name string) {
	reg.Register(name, Instance{Addr: s.addr})

	ln, _ := net.Listen("tcp", s.addr)
	for {
		conn, _ := ln.Accept()
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	for {
		msg, err := readMsg(conn)
		if err != nil {
			return
		}
		if !s.limiter.Allow() {
			continue
		}
		go s.process(conn, msg)
	}
}

func (s *Server) process(conn net.Conn, msg *Message) {
	method := reflect.ValueOf(s.srv).MethodByName(msg.Header.MethodName)

	arg := reflect.New(method.Type().In(0).Elem())
	reply := reflect.New(method.Type().In(1).Elem())

	json.Unmarshal(msg.Body, arg.Interface())

	method.Call([]reflect.Value{arg, reply})

	body, _ := json.Marshal(reply.Interface())
	resp := &Message{
		Header: &Header{RequestID: msg.Header.RequestID},
		Body:   body,
	}
	data, _ := encode(resp)
	conn.Write(data)
}

/////////////////////////////////////////////////////
// ================= CLIENT ========================
/////////////////////////////////////////////////////

type Client struct {
	reg     *Registry
	lb      LoadBalancer
	breaker CircuitBreaker
	seq     uint64
}

func NewClient(reg *Registry) *Client {
	return &Client{
		reg: reg,
		lb:  &RoundRobin{},
	}
}

func (c *Client) Invoke(ctx context.Context, service, method string, args interface{}, reply interface{}) error {

	return retry(3, func() error {

		if !c.breaker.Allow() {
			return fmt.Errorf("circuit open")
		}

		instances := c.reg.Get(service)
		if len(instances) == 0 {
			return fmt.Errorf("no instance")
		}

		target := c.lb.Select(instances)

		conn, err := net.Dial("tcp", target.Addr)
		if err != nil {
			c.breaker.RecordFailure()
			return err
		}
		defer conn.Close()

		id := atomic.AddUint64(&c.seq, 1)

		body, _ := json.Marshal(args)

		msg := &Message{
			Header: &Header{
				RequestID:   id,
				ServiceName: service,
				MethodName:  method,
			},
			Body: body,
		}

		data, _ := encode(msg)
		conn.Write(data)

		resp, err := readMsg(conn)
		if err != nil {
			c.breaker.RecordFailure()
			return err
		}

		json.Unmarshal(resp.Body, reply)
		c.breaker.RecordSuccess()
		return nil
	})
}

/////////////////////////////////////////////////////
// ================= DEMO ==========================
/////////////////////////////////////////////////////

type Args struct {
	A int
	B int
}

type Reply struct {
	Result int
}

type Arith struct{}

func (a *Arith) Add(args *Args, reply *Reply) {
	time.Sleep(200 * time.Millisecond)
	reply.Result = args.A + args.B
}

/////////////////////////////////////////////////////
// ================= MAIN ==========================
/////////////////////////////////////////////////////

func main() {

	reg := NewRegistry()

	// 启动两个 server
	go NewServer(":9001", &Arith{}).Start(reg, "Arith")
	go NewServer(":9002", &Arith{}).Start(reg, "Arith")

	time.Sleep(time.Second)

	client := NewClient(reg)

	for i := 0; i < 10; i++ {
		go func(i int) {
			args := &Args{A: i, B: i}
			reply := &Reply{}
			err := client.Invoke(context.Background(), "Arith", "Add", args, reply)
			if err != nil {
				fmt.Println("error:", err)
				return
			}
			fmt.Println("result:", reply.Result)
		}(i)
	}

	select {}
}
