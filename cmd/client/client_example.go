package main

import (
	"context"
	"fmt"
	"kamaRPC/internal/client"
	"kamaRPC/internal/codec"
	"kamaRPC/internal/registry"
	"kamaRPC/internal/transport"
	"kamaRPC/pkg/api"
	"log"
)

// func main() {
// 	reg, err := registry.NewRegistry([]string{"localhost:2379"})
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	client, err := client.NewClient(reg, client.WithClientCodec(codec.JSON))
// 	if err != nil {
// 		log.Println("client.NewClient:", err.Error())
// 		return
// 	}
// 	var wg sync.WaitGroup
// 	for i := 0; i < 10; i++ {
// 		wg.Add(1)
// 		go func(i int) {

// 			defer wg.Done()
// 			args := &api.Args{A: i, B: i}
// 			reply := &api.Reply{}

// 			err := client.Invoke(context.Background(), "Arith", "Add", args, reply)
// 			if err != nil {
// 				fmt.Println("error:", err)
// 				return
// 			}
// 			if args.A+args.B != reply.Result {
// 				log.Println("add 出现非法错误", args.A, " ", reply.Result)
// 			}
// 			fmt.Printf("Add %v+%v result: %v\n", args.A, args.B, reply.Result)
// 			args1 := &api.Args1{A: i, B: i, C: i}
// 			err = client.Invoke(context.Background(), "Arith2", "Mul", args1, reply)
// 			if err != nil {
// 				fmt.Println("error:", err)
// 				return
// 			}
// 			// if args.A*args.B != reply.Result {
// 			// 	log.Println("mul 出现非法错误", args.A, " ", reply.Result)
// 			// }
// 			fmt.Printf("mul %v*%v result: %v\n", args.A, args.B, reply.Result)
// 		}(i)
// 	}

// 	wg.Wait()

// }

func main() {
	reg, _ := registry.NewRegistry([]string{"localhost:2379"})

	c, _ := client.NewClient(reg, client.WithClientCodec(codec.JSON))

	type call struct {
		future *transport.Future
		args   *api.Args
	}

	var calls []call

	// 批量发送（不等待）
	for i := 0; i < 10; i++ {
		args := &api.Args{A: i, B: i}

		f, err := c.InvokeAsync(
			context.Background(),
			"Arith",
			"Add",
			args,
		)
		if err != nil {
			log.Println("send error:", err)
			continue
		}

		calls = append(calls, call{f, args})
	}

	// 统一等待
	for _, item := range calls {
		reply := &api.Reply{}

		err := item.future.GetResultWithContext(
			context.Background(),
			reply,
		)
		if err != nil {
			log.Println("get error:", err)
			continue
		}

		fmt.Printf("Add %v+%v result: %v\n",
			item.args.A,
			item.args.B,
			reply.Result)
	}
}
