package api

import "time"

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
