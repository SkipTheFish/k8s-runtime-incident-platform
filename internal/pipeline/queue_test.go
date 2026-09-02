package pipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"observability-platform/internal/signal"
)

func TestQueueRetriesThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	succeeded := make(chan struct{})
	q := NewQueue(Config{Workers: 1, MaxRetries: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}, func(context.Context, signal.Signal) error {
		if attempts.Add(1) < 3 {
			return errors.New("temporary")
		}
		close(succeeded)
		return nil
	}, nil)
	if err := q.Enqueue(signal.Signal{ID: "one"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = q.Run(ctx); close(done) }()
	select {
	case <-succeeded:
	case <-time.After(time.Second):
		t.Fatal("item did not succeed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queue did not shut down")
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts=%d, want 3", attempts.Load())
	}
	if err := q.Enqueue(signal.Signal{ID: "late"}); !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("expected shutdown error, got %v", err)
	}
}
