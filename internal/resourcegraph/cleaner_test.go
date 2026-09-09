package resourcegraph

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCleanerUsesRetentionAndStopsOnCancellation(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	called := make(chan time.Time, 1)
	cleaner := NewCleaner(24*time.Hour, time.Hour, time.Second, 25, func(_ context.Context, cutoff time.Time, batch int, timeout time.Duration) (int64, error) {
		if batch != 25 || timeout != time.Second {
			t.Errorf("batch=%d timeout=%v", batch, timeout)
		}
		called <- cutoff
		return 0, nil
	})
	cleaner.now = func() time.Time { return now }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- cleaner.Run(ctx) }()
	select {
	case cutoff := <-called:
		if !cutoff.Equal(now.Add(-24 * time.Hour)) {
			t.Fatalf("cutoff=%v", cutoff)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup did not run")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleaner did not stop")
	}
}

func TestCleanerReturnsCleanupError(t *testing.T) {
	want := errors.New("cleanup failed")
	cleaner := NewCleaner(time.Hour, time.Hour, time.Second, 1, func(context.Context, time.Time, int, time.Duration) (int64, error) { return 0, want })
	if err := cleaner.Run(context.Background()); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}
