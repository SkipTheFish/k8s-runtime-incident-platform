package event

import (
	"fmt"
	"sync"
	"time"
)

type bucket struct {
	count int
	reset time.Time
}

type RateLimiter struct {
	mu sync.Mutex

	buckets map[string]*bucket
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
	}
}

func (r *RateLimiter) Allow(evt RuntimeEvent) bool {

	key := r.key(evt)

	limit := r.limit(evt)

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	b, ok := r.buckets[key]

	if !ok || now.After(b.reset) {

		r.buckets[key] = &bucket{
			count: 1,
			reset: now.Add(time.Minute),
		}

		return true
	}

	if b.count >= limit {
		return false
	}

	b.count++

	return true
}

func (r *RateLimiter) limit(evt RuntimeEvent) int {

	switch evt.Type {

	case TypeK8sWarning:
		return 10

	case TypeK8sNormal:
		return 5

	default:
		return 30
	}
}

func (r *RateLimiter) key(evt RuntimeEvent) string {

	return fmt.Sprintf(
		"%s|%s|%s|%s",
		evt.Type,
		evt.Namespace,
		evt.Resource.Kind,
		evt.Resource.Name,
	)
}