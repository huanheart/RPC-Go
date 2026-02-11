package main

import (
	"context"
	"fmt"
	"kamaRPC/internal/client"
	"kamaRPC/internal/registry"
	"kamaRPC/pkg/api"
	"log"
	"sync"
)

func main() {
	reg, err := registry.NewRegistry([]string{"localhost:2379"})
	if err != nil {
		log.Fatal(err)
	}
	client := client.NewClient(reg)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {

			defer wg.Done()
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
	wg.Wait()

}
