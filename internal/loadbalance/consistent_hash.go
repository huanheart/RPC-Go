package loadbalance

import (
	"crypto/sha1"
	"encoding/binary"
	"kamaRPC/internal/registry"
)

type ConsistentHash struct{}

func (c *ConsistentHash) Select(list []registry.Instance) registry.Instance {
	if len(list) == 0 {
		return registry.Instance{}
	}

	// 简单示例，使用第一个服务名生成 hash
	key := list[0].Addr // 实际可用请求某个特征做 hash
	h := sha1.Sum([]byte(key))
	idx := binary.BigEndian.Uint32(h[:4]) % uint32(len(list))
	return list[idx]
}
