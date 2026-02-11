package loadbalance

import (
	"kamaRPC/internal/registry"
	"sync"
)

type WeightedRR struct {
	idx       uint64
	currCount uint64
	mu        sync.Mutex
	weights   []int
}

func NewWeightedRR(weights []int) *WeightedRR {
	return &WeightedRR{
		weights: weights,
	}
}

func (w *WeightedRR) Select(list []registry.Instance) registry.Instance {
	if len(list) == 0 {
		return registry.Instance{}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 轮询加权
	for {
		w.idx = (w.idx + 1) % uint64(len(list))
		if w.currCount < uint64(w.weights[w.idx]) {
			w.currCount++
			return list[w.idx]
		} else {
			w.currCount = 0
		}
	}
}
