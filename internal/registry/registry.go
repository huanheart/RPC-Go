package registry

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

/////////////////////////////////////////////////////
// ================= REGISTRY ======================
/////////////////////////////////////////////////////

type Instance struct {
	Addr string
}

type Registry struct {
	client *clientv3.Client
	prefix string
}

/////////////////////////////////////////////////////
// =============== 初始化 ==========================
/////////////////////////////////////////////////////

func NewRegistry(endpoints []string) (*Registry, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &Registry{
		client: cli,
		prefix: "/kamaRPC/services/",
	}, nil
}

/////////////////////////////////////////////////////
// ================= 注册服务 ======================
/////////////////////////////////////////////////////

func (r *Registry) Register(service string, ins Instance, ttl int64) error {
	ctx := context.Background()

	// 1️⃣ 创建租约
	leaseResp, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s%s/%s", r.prefix, service, ins.Addr)

	// 2️⃣ 写入 etcd，并绑定租约
	_, err = r.client.Put(ctx, key, ins.Addr, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return err
	}

	// 3️⃣ 开启自动续约（心跳）
	ch, err := r.client.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		return err
	}

	// 后台消费续约响应
	go func() {
		for {
			<-ch
		}
	}()

	return nil
}

/////////////////////////////////////////////////////
// ================= 服务发现 ======================
/////////////////////////////////////////////////////

func (r *Registry) Discover(service string) ([]Instance, error) {
	ctx := context.Background()

	key := fmt.Sprintf("%s%s/", r.prefix, service)

	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var instances []Instance
	for _, kv := range resp.Kvs {
		instances = append(instances, Instance{
			Addr: string(kv.Value),
		})
	}

	return instances, nil
}

/////////////////////////////////////////////////////
// ================= 关闭连接 ======================
/////////////////////////////////////////////////////

func (r *Registry) Close() error {
	return r.client.Close()
}
