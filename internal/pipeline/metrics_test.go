package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"observability-platform/internal/signal"
)

func TestPipelineMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	dead := make(chan struct{})
	q := NewQueue(Config{Workers: 1, MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}, func(context.Context, signal.Signal) error { return errors.New("permanent") }, func(context.Context, signal.Signal, error, int) { close(dead) })
	m := NewMetrics(registry, q.Depth)
	q.SetMetrics(m)
	item := signal.Signal{ID: "id", Source: signal.SourceKubernetes, Type: signal.TypeWarning}
	if err := q.Enqueue(item); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = q.Run(ctx); close(done) }()
	select {
	case <-dead:
	case <-time.After(time.Second):
		t.Fatal("dead letter not reached")
	}
	if got := testutil.ToFloat64(m.received.With(labels("kubernetes", "warning"))); got != 1 {
		t.Fatalf("received=%v", got)
	}
	if got := testutil.ToFloat64(m.retries.With(labels("kubernetes", "warning"))); got != 1 {
		t.Fatalf("retries=%v", got)
	}
	if got := testutil.ToFloat64(m.failed.With(labels("kubernetes", "warning"))); got != 1 {
		t.Fatalf("failed=%v", got)
	}
	cancel()
	<-done
}
