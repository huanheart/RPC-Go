package main

import (
	"kamaRPC/internal/registry"
	"kamaRPC/internal/server"
	"kamaRPC/pkg/api"
	"log"
)

func main() {
	reg, err := registry.NewRegistry([]string{"localhost:2379"})
	if err != nil {
		log.Fatal(err)
	}

	srv := server.NewServer(":9090")

	// 注册 Arith 服务
	srv.Register("Arith", &api.Arith{})

	// 注册服务到 etcd
	err = reg.Register("Arith", registry.Instance{
		Addr: "localhost:9090",
	}, 10)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("server started at :9090")
	srv.Start()
}
