package server

import (
	"kamaRPC/internal/limiter"
	"kamaRPC/internal/registry"
	"net"
)

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
		go s.Handle(conn)
	}
}
