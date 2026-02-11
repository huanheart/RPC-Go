package server

import (
	"kamaRPC/internal/limiter"
	"net"
)

type Server struct {
	addr     string
	services map[string]interface{}
	limiter  *limiter.TokenBucket
}

func NewServer(addr string) *Server {
	return &Server{
		addr:     addr,
		services: make(map[string]interface{}),
		limiter:  limiter.NewTokenBucket(100),
	}
}

func (s *Server) Register(name string, service interface{}) {
	s.services[name] = service
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go s.Handle(conn)
	}
}
