package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Deliverer interface {
	Deliver(context.Context, Event) error
}

type Config struct {
	BatchSize       int
	PollInterval    time.Duration
	Lease           time.Duration
	DeliveryTimeout time.Duration
}

type Dispatcher struct {
	store     *Store
	deliverer Deliverer
	config    Config
}

func NewDispatcher(store *Store, deliverer Deliverer, config Config) *Dispatcher {
	if config.BatchSize < 1 {
		config.BatchSize = 20
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if config.Lease <= 0 {
		config.Lease = 30 * time.Second
	}
	if config.DeliveryTimeout <= 0 {
		config.DeliveryTimeout = 10 * time.Second
	}
	return &Dispatcher{store: store, deliverer: deliverer, config: config}
}

func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := d.ProcessOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) ProcessOnce(ctx context.Context) (int, error) {
	events, err := d.store.ClaimBatch(ctx, d.config.BatchSize, d.config.Lease)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		deliveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.config.DeliveryTimeout)
		if len(event.Payload) > MaxPayloadBytes {
			err = fmt.Errorf("outbox payload exceeds %d bytes", MaxPayloadBytes)
		} else {
			err = d.deliverer.Deliver(deliveryCtx, event)
		}
		cancel()
		if err != nil {
			if releaseErr := d.store.Release(context.WithoutCancel(ctx), event.EventID, err); releaseErr != nil {
				return 0, releaseErr
			}
			continue
		}
		if err := d.store.MarkDelivered(context.WithoutCancel(ctx), event.EventID); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}
