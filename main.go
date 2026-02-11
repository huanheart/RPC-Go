package main

import (
	"context"
	"fmt"
	"kamaRPC/internal/client"
	"kamaRPC/internal/registry"
	"kamaRPC/internal/server"
	"kamaRPC/pkg/api"
	"time"
)

// ///////////////////////////////////////////////////
// ================= MAIN ==========================
// ///////////////////////////////////////////////////
// test 注释
func main() {

	reg := registry.NewRegistry()

	go server.NewServer(":9001", &api.Arith{}).Start(reg, "Arith")
	go server.NewServer(":9002", &api.Arith{}).Start(reg, "Arith")

	time.Sleep(time.Second)

	client := client.NewClient(reg)

	for i := 0; i < 10; i++ {
		go func(i int) {
			args := &api.Args{A: i, B: i}
			reply := &api.Reply{}

			err := client.Invoke(context.Background(), "Arith", "Add", args, reply)
			if err != nil {
				fmt.Println("error:", err)
				return
			}

			fmt.Println("result:", reply.Result)
		}(i)
	}

	select {}
}
