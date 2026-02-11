package main

import (
	"context"
	"encoding/json"
	"fmt"
	"kamaRPC/internal/breaker"
	"kamaRPC/internal/limiter"
	"kamaRPC/internal/loadbalance"
	"kamaRPC/internal/protocol"
	"kamaRPC/internal/registry"
	"kamaRPC/internal/retry"
	"net"
	"reflect"
	"sync/atomic"
	"time"
)

/////////////////////////////////////////////////////
// ================= LOAD BALANCE ==================
/////////////////////////////////////////////////////

/////////////////////////////////////////////////////
// ================= RETRY =========================
/////////////////////////////////////////////////////

/////////////////////////////////////////////////////
// ================= CIRCUIT BREAKER ==============
/////////////////////////////////////////////////////

/////////////////////////////////////////////////////
// ================= LIMITER =======================
/////////////////////////////////////////////////////

/////////////////////////////////////////////////////
// ================= SERVER ========================
/////////////////////////////////////////////////////

type Server struct {
	addr    string
	srv     interface{}
	limiter *limiter.TokenBucket
}

func NewServer(addr string, srv interface{}) *Server {
	return &Server{
		addr:    addr,
		srv:     srv,
		limiter: limiter.NewTokenBucket(5),
	}
}

func (s *Server) Start(reg *registry.Registry, name string) {
	reg.Register(name, registry.Instance{Addr: s.addr})

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

	data, _ := protocol.Encode(resp)
	conn.Write(data)
}

/////////////////////////////////////////////////////
// ================= CLIENT ========================
/////////////////////////////////////////////////////

type Client struct {
	reg     *registry.Registry
	lb      loadbalance.LoadBalancer
	breaker breaker.CircuitBreaker
	seq     uint64
}

func NewClient(reg *registry.Registry) *Client {
	return &Client{
		reg: reg,
		lb:  &loadbalance.RoundRobin{},
	}
}

func (c *Client) Invoke(ctx context.Context, service, method string, args interface{}, reply interface{}) error {

	return retry.Retry(3, func() error {

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

	reg := registry.NewRegistry()

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
