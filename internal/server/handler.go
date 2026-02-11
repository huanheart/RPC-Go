package server

import (
	"kamaRPC/internal/protocol"
	"net"
)

func (s *Server) Handle(conn net.Conn) {
	defer conn.Close()

	for {
		msg, err := protocol.Decode(conn)
		if err != nil {
			return
		}

		if !s.limiter.Allow() {
			continue
		}

		go s.Process(conn, msg)
	}
}
