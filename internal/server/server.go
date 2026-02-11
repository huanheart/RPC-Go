package server

import (
	"kamaRPC/internal/limiter"
	"kamaRPC/internal/protocol"
	"kamaRPC/internal/transport"
	"net"
)

type Server struct {
	addr     string
	services map[string]interface{}
	limiter  *limiter.TokenBucket
	listener net.Listener
	handler  *Handler
}

func NewServer(addr string) *Server {
	s := &Server{
		addr:     addr,
		services: make(map[string]interface{}),
		limiter:  limiter.NewTokenBucket(100),
		handler:  NewHandler(nil),
	}
	return s
}

func (s *Server) Register(name string, service interface{}) {
	s.services[name] = service
}

func (s *Server) Handle(conn *transport.TCPConnection) {
	defer conn.Close()

	for {
		// 读取请求
		msg, err := conn.Read()
		if err != nil {
			// 连接被关闭或出错，退出
			return
		}

		// 限流检查
		if !s.limiter.Allow() {
			resp := &protocol.Message{
				Header: &protocol.Header{
					RequestID: msg.Header.RequestID,
					Error:     "rate limit exceeded",
				},
			}
			conn.Write(resp)
			continue
		}

		// 更新 handler 中的服务
		s.handler.server = s.services[msg.Header.ServiceName]

		// 处理请求
		s.handler.Process(conn, msg)
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		tcpConn := transport.NewTCPConnection(conn)
		go s.Handle(tcpConn)
	}
}

func (s *Server) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
}
