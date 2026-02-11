package client

import (
	"context"
	"encoding/json"
	"errors"
	"kamaRPC/internal/protocol"
	"net"
)

func (c *Client) Invoke(ctx context.Context, service, method string, args interface{}, reply interface{}) error {
	instances, err := c.reg.Discover(service)
	if err != nil {
		return err
	}

	if len(instances) == 0 {
		return errors.New("no instance")
	}

	addr := instances[0].Addr

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 使用正确的协议格式发送请求
	body, _ := json.Marshal(args)
	req := &protocol.Message{
		Header: &protocol.Header{
			ServiceName: service,
			MethodName:  method,
		},
		Body: body,
	}

	data, _ := protocol.Encode(req)
	conn.Write(data)

	// 使用协议读取响应
	resp, err := protocol.Decode(conn)
	if err != nil {
		return err
	}

	return json.Unmarshal(resp.Body, reply)
}
