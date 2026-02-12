package server

import (
	"encoding/json"
	"fmt"
	"kamaRPC/internal/codec"
	"kamaRPC/internal/protocol"
	"kamaRPC/internal/transport"
	"log"
	"reflect"
)

// Handler 处理请求
type Handler struct {
	server interface{}
	codec  codec.Codec
}

func NewHandler(s interface{}, opts ...HandleOption) (*Handler, error) {
	h := &Handler{server: s}

	for _, opt := range opts {
		if err := opt(h); err != nil {
			return nil, err
		}
	}

	return h, nil
}

// Process 处理 TCP 请求
func (h *Handler) Process(conn *transport.TCPConnection, msg *protocol.Message) {
	service := h.server
	if service == nil {
		resp := &protocol.Message{
			Header: &protocol.Header{
				RequestID: msg.Header.RequestID,
				Error:     "service not found",
			},
		}
		conn.Write(resp)
		return
	}

	result, err := h.invoke(service, msg.Header.ServiceName, msg.Header.MethodName, msg.Body)
	if err != nil {
		resp := &protocol.Message{
			Header: &protocol.Header{
				RequestID: msg.Header.RequestID,
				Error:     err.Error(),
			},
		}
		conn.Write(resp)
		return
	}

	body, err := h.codec.Marshal(result)
	if err != nil {
		log.Println("Handler Process failed err ", err.Error())
	}
	resp := &protocol.Message{
		Header: &protocol.Header{
			RequestID: msg.Header.RequestID,
			Error:     "",
		},
		Body: body,
	}
	conn.Write(resp)
}

// invoke 使用反射调用服务方法
func (h *Handler) invoke(service interface{}, serviceName, methodName string, body []byte) (interface{}, error) {
	serviceValue := reflect.ValueOf(service)
	methodValue := serviceValue.MethodByName(methodName)
	if !methodValue.IsValid() {
		return nil, fmt.Errorf("method not found: %s.%s", serviceName, methodName)
	}

	numIn := methodValue.Type().NumIn()
	args := make([]reflect.Value, 0, numIn)

	if numIn == 1 {
		// 单参数方法
		argType := methodValue.Type().In(0)
		arg := reflect.New(argType.Elem()) // 假设是 *T
		if len(body) > 0 {
			if err := json.Unmarshal(body, arg.Interface()); err != nil {
				return nil, err
			}
		}
		args = append(args, arg)
	} else if numIn == 2 {
		// 标准 RPC 方法 func(arg *Arg, reply *Reply) error
		// 处理第一个参数
		argType := methodValue.Type().In(0)
		arg := reflect.New(argType.Elem())
		if len(body) > 0 {
			if err := json.Unmarshal(body, arg.Interface()); err != nil {
				return nil, err
			}
		}
		args = append(args, arg)

		// 处理第二个参数 reply
		replyType := methodValue.Type().In(1)
		reply := reflect.New(replyType.Elem()) // 避免 **T
		args = append(args, reply)
	} else if numIn > 2 {
		// 多参数方法
		for i := 0; i < numIn; i++ {
			paramType := methodValue.Type().In(i)
			param := reflect.New(paramType.Elem())
			if i == 0 && len(body) > 0 {
				_ = json.Unmarshal(body, param.Interface())
			}
			args = append(args, param)
		}
	}

	// 调用方法
	results := methodValue.Call(args)

	// 如果是标准 RPC，有 reply 参数，返回 reply
	if numIn == 2 {
		reply := args[1]
		return reply.Interface(), nil
	}

	if len(results) > 0 {
		return results[0].Interface(), nil
	}

	return nil, nil
}
