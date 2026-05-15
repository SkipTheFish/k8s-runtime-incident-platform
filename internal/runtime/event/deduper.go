package event

import (
	"crypto/sha1"
	"fmt"
	"sync"
	"time"
)

type Deduper struct {
	mu sync.Mutex

	cache map[string]time.Time
}

func NewDeduper() *Deduper {
	return &Deduper{
		cache: make(map[string]time.Time),
	}
}

func (d *Deduper) Seen(evt RuntimeEvent) bool {

	key := d.key(evt)

	ttl := d.ttl(evt)

	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	// cleanup
	for k, t := range d.cache {
		if now.Sub(t) > ttl {
			delete(d.cache, k)
		}
	}

	// already exists
	if t, ok := d.cache[key]; ok {
		if now.Sub(t) <= ttl {
			return true
		}
	}

	d.cache[key] = now

	return false
}

func (d *Deduper) ttl(evt RuntimeEvent) time.Duration {

	switch evt.Type {

	case TypeK8sWarning:
		return 2 * time.Minute

	case TypeK8sNormal:
		return 30 * time.Second

	case TypeAlertTriggered:
		return 15 * time.Second

	case TypeAlertResolved:
		return 15 * time.Second

	default:
		return 1 * time.Minute
	}
}

func (d *Deduper) key(evt RuntimeEvent) string {

	raw := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s",
		evt.Type,
		evt.Namespace,
		evt.Resource.Kind,
		evt.Resource.Name,
		evt.PolicyName,
		evt.Message,
	)

	return fmt.Sprintf("%x", sha1.Sum([]byte(raw)))
}