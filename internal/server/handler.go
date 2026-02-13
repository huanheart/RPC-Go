package server

import (
	"context"
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
	h := &Handler{
		server: s,
	}

	for _, opt := range opts {
		if err := opt(h); err != nil {
			return nil, err
		}
	}

	if h.codec == nil {
		return nil, fmt.Errorf("codec must not be nil")
	}

	return h, nil
}

func (h *Handler) Process(conn *transport.TCPConnection, msg *protocol.Message) {
	if h.server == nil {
		h.writeError(conn, msg.Header.RequestID, "service not found")
		return
	}

	result, err := h.invoke(
		context.Background(),
		h.server,
		msg.Header.ServiceName,
		msg.Header.MethodName,
		msg.Body,
	)

	if err != nil {
		h.writeError(conn, msg.Header.RequestID, err.Error())
		return
	}

	var body []byte
	if result != nil {
		var marshalErr error
		body, marshalErr = h.codec.Marshal(result)
		if marshalErr != nil {
			log.Println("marshal error:", marshalErr)
			h.writeError(conn, msg.Header.RequestID, marshalErr.Error())
			return
		}
	}

	resp := &protocol.Message{
		Header: &protocol.Header{
			RequestID: msg.Header.RequestID,
		},
		Body: body,
	}

	conn.Write(resp)
}

func (h *Handler) writeError(conn *transport.TCPConnection, requestID uint64, errMsg string) {
	resp := &protocol.Message{
		Header: &protocol.Header{
			RequestID: requestID,
			Error:     errMsg,
		},
	}
	conn.Write(resp)
}

func (h *Handler) invoke(
	ctx context.Context,
	service interface{},
	serviceName,
	methodName string,
	body []byte,
) (interface{}, error) {

	serviceValue := reflect.ValueOf(service)
	method := serviceValue.MethodByName(methodName)
	if !method.IsValid() {
		return nil, fmt.Errorf("method not found: %s.%s", serviceName, methodName)
	}

	methodType := method.Type()
	numIn := methodType.NumIn()
	numOut := methodType.NumOut()

	args := make([]reflect.Value, 0, numIn)

	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()

	// =========================
	// 1️⃣ net/rpc 风格
	// func(req *Req, reply *Resp) error
	// =========================
	if numIn == 2 &&
		methodType.In(0).Kind() == reflect.Ptr &&
		methodType.In(1).Kind() == reflect.Ptr &&
		numOut == 1 &&
		methodType.Out(0).Implements(reflect.TypeOf((*error)(nil)).Elem()) {

		// 构造 req
		reqType := methodType.In(0)
		req := reflect.New(reqType.Elem())

		if len(body) > 0 {
			if err := h.codec.Unmarshal(body, req.Interface()); err != nil {
				return nil, err
			}
		}

		// 构造 reply
		replyType := methodType.In(1)
		reply := reflect.New(replyType.Elem())

		args = append(args, req)
		args = append(args, reply)

		results := method.Call(args)

		// 处理 error
		if errVal := results[0].Interface(); errVal != nil {
			return nil, errVal.(error)
		}

		// 🔥 关键修复：必须 Elem()
		return reply.Elem().Interface(), nil
	}

	// =========================
	// 2️⃣ 推荐生产签名
	// func(ctx context.Context, req *Req) (*Resp, error)
	// func(req *Req) (*Resp, error)
	// =========================

	for i := 0; i < numIn; i++ {
		paramType := methodType.In(i)

		// context
		if paramType.Implements(contextType) {
			args = append(args, reflect.ValueOf(ctx))
			continue
		}

		// 业务参数
		if paramType.Kind() == reflect.Ptr {
			// reqIndex = i
			req := reflect.New(paramType.Elem())

			if len(body) > 0 {
				if err := h.codec.Unmarshal(body, req.Interface()); err != nil {
					return nil, err
				}
			}

			args = append(args, req)
			continue
		}

		return nil, fmt.Errorf("unsupported param type: %s", paramType.String())
	}

	results := method.Call(args)

	// =========================
	// 处理返回值
	// =========================

	switch numOut {

	case 0:
		return nil, nil

	case 1:
		// 可能是 error
		if err, ok := results[0].Interface().(error); ok {
			return nil, err
		}
		return results[0].Interface(), nil

	case 2:
		// (result, error)
		if errVal := results[1].Interface(); errVal != nil {
			return nil, errVal.(error)
		}
		return results[0].Interface(), nil

	default:
		return nil, fmt.Errorf("unsupported return signature")
	}
}
