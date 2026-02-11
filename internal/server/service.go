package server

import (
	"encoding/json"
	"kamaRPC/internal/protocol"
	"net"
	"reflect"
)

func (s *Server) Process(conn net.Conn, msg *protocol.Message) {

	svc, ok := s.services[msg.Header.ServiceName]
	if !ok {
		return
	}

	method := reflect.ValueOf(svc).MethodByName(msg.Header.MethodName)
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
