package resilience

import (
	"errors"
	"sync"
	"time"
)

type CircuitBreaker struct {
	failures int
	mu       sync.Mutex
	open     bool
	lastFail time.Time
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	if cb.open && time.Since(cb.lastFail) < 5*time.Second {
		cb.mu.Unlock()
		return errors.New("circuit open")
	}

	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFail = time.Now()

		if cb.failures >= 3 {
			cb.open = true
		}
	} else {
		cb.failures = 0
		cb.open = false
	}

	return err
}