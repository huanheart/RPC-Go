package main

import (
	"context"
	"encoding/json"
	"fmt"
	"kamaRPC/internal/protocol"
	"math"
	"math/rand"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

/////////////////////////////////////////////////////
// ================= REGISTRY ======================
/////////////////////////////////////////////////////

type Instance struct {
	Addr string
}

type Registry struct {
	mu       sync.RWMutex
	services map[string][]Instance
}

func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string][]Instance),
	}
}

func (r *Registry) Register(service string, ins Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[service] = append(r.services[service], ins)
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

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	for {
		msg, err := protocol.Decode(conn)
		if err != nil {
			return
		}

		if !s.limiter.Allow() {
			continue
		}

		go s.process(conn, msg)
	}
}

func (s *Server) process(conn net.Conn, msg *protocol.Message) {

	method := reflect.ValueOf(s.srv).MethodByName(msg.Header.MethodName)
	if !method.IsValid() {
		return
	}

	arg := reflect.New(method.Type().In(0).Elem())
	reply := reflect.New(method.Type().In(1).Elem())

	if err := json.Unmarshal(msg.Body, arg.Interface()); err != nil {
		return
	}

	method.Call([]reflect.Value{arg, reply})

	body, _ := json.Marshal(reply.Interface())

	resp := &protocol.Message{
		Header: &protocol.Header{
			RequestID: msg.Header.RequestID,
		},
		Body: body,
	}

	// ✅ 修复：必须编码 resp
	data, _ := protocol.Encode(resp)
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

		msg := &protocol.Message{
			Header: &protocol.Header{
				RequestID:   id,
				ServiceName: service,
				MethodName:  method,
			},
			Body: body,
		}

		data, _ := protocol.Encode(msg)
		conn.Write(data)

		resp, err := protocol.Decode(conn)
		if err != nil {
			c.breaker.RecordFailure()
			return err
		}

		if err := json.Unmarshal(resp.Body, reply); err != nil {
			return err
		}

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
