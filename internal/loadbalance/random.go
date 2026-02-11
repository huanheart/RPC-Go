package loadbalance

import (
	"kamaRPC/internal/registry"
	"math/rand"
	"time"
)

type Random struct{}

func (r *Random) Select(list []registry.Instance) registry.Instance {
	if len(list) == 0 {
		return registry.Instance{}
	}
	rand.Seed(time.Now().UnixNano())
	return list[rand.Intn(len(list))]
}
