package loadbalance

import (
	"kamaRPC/internal/registry"
	"sync/atomic"
)

type RoundRobin struct {
	idx uint64
}

func (r *RoundRobin) Select(list []registry.Instance) registry.Instance {
	i := atomic.AddUint64(&r.idx, 1)
	return list[i%uint64(len(list))]
}
