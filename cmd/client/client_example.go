package main

import (
	"context"
	"fmt"
	"kamaRPC/internal/client"
	"kamaRPC/internal/registry"
	"kamaRPC/pkg/api"
	"log"
	"time"
)

func main() {
	reg, err := registry.NewRegistry([]string{"localhost:2379"})
	if err != nil {
		log.Fatal(err)
	}
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

	time.Sleep(5 * time.Second)
}
