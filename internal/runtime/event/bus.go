package event

import (
	"context"
	"log"
	"sync"
)

type Bus struct {
	mu sync.RWMutex

	subscribers []chan RuntimeEvent

	validator Validator

	deduper *Deduper

	rateLimiter *RateLimiter

	metrics *Metrics
}

func NewBus() *Bus {

	return &Bus{
		validator: NewDefaultValidator(),

		deduper: NewDeduper(),

		rateLimiter: NewRateLimiter(),

		metrics: NewMetrics(),
	}
}

func (b *Bus) Subscribe(buffer int) <-chan RuntimeEvent {

	ch := make(chan RuntimeEvent, buffer)

	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()

	return ch
}

func (b *Bus) Publish(
	ctx context.Context,
	evt RuntimeEvent,
) error {

	// =========================================
	// Validation
	// =========================================

	if err := b.validator.Validate(evt); err != nil {

		b.metrics.IncValidationFailed()

		log.Printf(
			"EVENT BUS: validation failed err=%v",
			err,
		)

		return err
	}

	// =========================================
	// Dedup
	// =========================================

	if b.deduper.Seen(evt) {

		b.metrics.IncDeduped()

		log.Printf(
			"EVENT BUS: deduped type=%s namespace=%s resource=%s/%s policy=%s",
			evt.Type,
			evt.Namespace,
			evt.Resource.Kind,
			evt.Resource.Name,
			evt.PolicyName,
		)

		return ErrDuplicated
	}

	// =========================================
	// Rate Limit
	// =========================================

	if !b.rateLimiter.Allow(evt) {

		b.metrics.IncRateLimited()

		log.Printf(
			"EVENT BUS: rate limited type=%s namespace=%s resource=%s/%s",
			evt.Type,
			evt.Namespace,
			evt.Resource.Kind,
			evt.Resource.Name,
		)

		return ErrRateLimited
	}

	// =========================================
	// Publish
	// =========================================

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {

		select {

		case ch <- evt:

		case <-ctx.Done():
			return ctx.Err()

		default:

			b.metrics.IncDropped()

			log.Printf(
				"EVENT BUS: dropped event type=%s source=%s",
				evt.Type,
				evt.Source,
			)
		}
	}

	b.metrics.IncPublished()

	log.Printf(
		"EVENT BUS: published type=%s namespace=%s source=%s",
		evt.Type,
		evt.Namespace,
		evt.Source,
	)

	return nil
}

func (b *Bus) Status() Status {

	b.mu.RLock()
	defer b.mu.RUnlock()

	return Status{
		Subscribers: len(b.subscribers),

		Metrics: b.metrics.Snapshot(),
	}
}
