package breaker

import (
	"sync"
	"time"
)

type CircuitBreaker struct {
	failures int
	state    int // 0 closed, 1 open
	mu       sync.Mutex
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == 0
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	if cb.failures > 3 {
		cb.state = 1
		go func() {
			time.Sleep(2 * time.Second)
			cb.mu.Lock()
			cb.state = 0
			cb.failures = 0
			cb.mu.Unlock()
		}()
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	cb.failures = 0
	cb.mu.Unlock()
}
