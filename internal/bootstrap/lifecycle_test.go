package bootstrap

import (
	"context"
	"errors"
	"strings"
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
