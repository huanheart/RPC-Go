package client

import (
	"context"
	"encoding/json"
	"fmt"
	"kamaRPC/internal/protocol"
	"kamaRPC/internal/retry"
	"net"
	"sync/atomic"
)

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
