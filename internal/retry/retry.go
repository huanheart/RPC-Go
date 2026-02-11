package retry

import (
	"math"
	"math/rand"
	"time"
)

func Retry(attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		backoff := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
		jitter := time.Duration(rand.Intn(100)) * time.Millisecond
		time.Sleep(backoff + jitter)
	}
	return err
}
