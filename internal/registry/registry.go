package registry

import "sync"

/////////////////////////////////////////////////////
// ================= REGISTRY ======================
/////////////////////////////////////////////////////

type Instance struct {
	Addr string
}

type Registry struct {
	mu       sync.RWMutex
	services map[string][]Instance
}

func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string][]Instance),
	}
}

func (r *Registry) Register(service string, ins Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[service] = append(r.services[service], ins)
}

func (r *Registry) Get(service string) []Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.services[service]
}
