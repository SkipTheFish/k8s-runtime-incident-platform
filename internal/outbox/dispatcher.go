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
type DeliveryFailure interface {
	error
	Retryable() bool
	HTTPStatus() int
	ResponseSummary() string
	RetryAfter() time.Duration
}

type Config struct {
	BatchSize                            int
	PollInterval, Lease, DeliveryTimeout time.Duration
	TargetName                           string
	MaxAttempts                          int
	BaseDelay, MaxDelay                  time.Duration
}

type Dispatcher struct {
	store     *Store
	deliverer Deliverer
	config    Config
	now       func() time.Time
	random    func() float64
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
	if config.TargetName == "" {
		config.TargetName = "webhook"
	}
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 5
	}
	if config.BaseDelay <= 0 {
		config.BaseDelay = time.Second
	}
	if config.MaxDelay < config.BaseDelay {
		config.MaxDelay = time.Minute
	}
	return &Dispatcher{store: store, deliverer: deliverer, config: config, now: time.Now, random: func() float64 { return 0.5 }}
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
			retryable, status, summary, retryAfter := deliveryFailure(err)
			deadLetter := !retryable || event.Attempt >= d.config.MaxAttempts
			next := d.now().Add(d.backoff(event.Attempt, retryAfter))
			if recordErr := d.store.RecordAttempt(context.WithoutCancel(ctx), event, d.config.TargetName, "retryable", status, summary, next, deadLetter); recordErr != nil {
				return 0, recordErr
			}
			continue
		}
		if err := d.store.RecordAttempt(context.WithoutCancel(ctx), event, d.config.TargetName, "succeeded", 0, "", time.Time{}, false); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}

func (d *Dispatcher) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > d.config.MaxDelay {
			return d.config.MaxDelay
		}
		return retryAfter
	}
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	delay := d.config.BaseDelay << shift
	if delay > d.config.MaxDelay {
		delay = d.config.MaxDelay
	}
	return time.Duration(float64(delay) * (0.5 + d.random()))
}

func deliveryFailure(err error) (bool, int, string, time.Duration) {
	var failure DeliveryFailure
	if errors.As(err, &failure) {
		return failure.Retryable(), failure.HTTPStatus(), failure.ResponseSummary(), failure.RetryAfter()
	}
	return true, 0, err.Error(), 0
}
