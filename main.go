package main

import (
	"context"
	"fmt"
	"kamaRPC/internal/client"
	"kamaRPC/internal/registry"
	"kamaRPC/internal/server"
	"time"
)

/////////////////////////////////////////////////////
// ================= DEMO ==========================
/////////////////////////////////////////////////////

type Args struct {
	A int
	B int
}

type Reply struct {
	Result int
}

type Arith struct{}

func (a *Arith) Add(args *Args, reply *Reply) {
	time.Sleep(200 * time.Millisecond)
	reply.Result = args.A + args.B
}

/////////////////////////////////////////////////////
// ================= MAIN ==========================
/////////////////////////////////////////////////////

func main() {

	reg := registry.NewRegistry()

	go server.NewServer(":9001", &Arith{}).Start(reg, "Arith")
	go server.NewServer(":9002", &Arith{}).Start(reg, "Arith")

	time.Sleep(time.Second)

	client := client.NewClient(reg)

	for i := 0; i < 10; i++ {
		go func(i int) {
			args := &Args{A: i, B: i}
			reply := &Reply{}

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
