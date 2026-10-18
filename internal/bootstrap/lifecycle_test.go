package bootstrap

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestComponentErrorCancelsSiblings(t *testing.T) {
	cancelled := make(chan struct{})
	components := []Component{
		componentFunc{name: "failing", run: func(context.Context) error { return errors.New("boom") }},
		componentFunc{name: "waiting", run: func(ctx context.Context) error {
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		}},
	}
	err := runComponents(context.Background(), time.Second, components)
	if err == nil || !strings.Contains(err.Error(), "failing: boom") {
		t.Fatalf("expected component error, got %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("sibling component was not cancelled")
	}
}

func TestCancellationStopsAllManagedComponentClasses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var stopped atomic.Int32
	names := []string{"http", "controller-manager", "informer", "worker", "dispatcher"}
	components := make([]Component, 0, len(names))
	for _, name := range names {
		components = append(components, componentFunc{name: name, run: func(ctx context.Context) error {
			<-ctx.Done()
			stopped.Add(1)
			return ctx.Err()
		}})
	}
	done := make(chan error, 1)
	go func() { done <- runComponents(ctx, 500*time.Millisecond, components) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("managed components exceeded graceful shutdown bound")
	}
	if got := stopped.Load(); got != int32(len(names)) {
		t.Fatalf("stopped=%d want=%d", got, len(names))
	}
}

func TestShutdownTimeoutIsBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	components := []Component{componentFunc{name: "stuck", run: func(context.Context) error { select {} }}}
	go cancel()
	started := time.Now()
	err := runComponents(ctx, 20*time.Millisecond, components)
	if err == nil || !strings.Contains(err.Error(), "shutdown exceeded") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("shutdown timeout was not bounded")
	}
}
