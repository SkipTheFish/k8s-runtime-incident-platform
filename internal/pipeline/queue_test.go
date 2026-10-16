package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

func TestQueueConcurrentProducersAndWorkers(t *testing.T) {
	const total = 200
	var handled sync.Map
	var count atomic.Int32
	completed := make(chan struct{})
	q := NewQueue(Config{Workers: 8, MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}, func(_ context.Context, item signal.Signal) error {
		if _, loaded := handled.LoadOrStore(item.ID, struct{}{}); loaded {
			t.Errorf("signal %s handled more than once", item.ID)
		}
		if count.Add(1) == total {
			close(completed)
		}
		return nil
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = q.Run(ctx); close(done) }()
	var producers sync.WaitGroup
	for producer := 0; producer < 10; producer++ {
		producers.Add(1)
		go func(producer int) {
			defer producers.Done()
			for index := 0; index < total/10; index++ {
				if err := q.Enqueue(signal.Signal{ID: fmt.Sprintf("%d-%d", producer, index)}); err != nil {
					t.Errorf("enqueue: %v", err)
				}
			}
		}(producer)
	}
	producers.Wait()
	select {
	case <-completed:
	case <-time.After(5 * time.Second):
		t.Fatalf("handled=%d want=%d", count.Load(), total)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("queue did not stop")
	}
}
