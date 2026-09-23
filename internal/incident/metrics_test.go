package incident

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestIncidentLifecycleMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.created.Inc()
	metrics.reopened.Add(2)
	metrics.resolved.Add(3)
	for name, test := range map[string]struct {
		collector prometheus.Collector
		want      float64
	}{
		"created":  {metrics.created, 1},
		"reopened": {metrics.reopened, 2},
		"resolved": {metrics.resolved, 3},
	} {
		if got := testutil.ToFloat64(test.collector); got != test.want {
			t.Errorf("%s=%v want=%v", name, got, test.want)
		}
	}
}

func TestLifecycleMetricsOnlyAfterCommittedChanges(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	metrics := NewMetrics(prometheus.NewRegistry())
	service := NewTransactionService(pool, 3*time.Second).WithMetrics(metrics)
	match := incidentMatch()
	base := time.Now().UTC().Add(-time.Minute)
	item := incidentSignal("metrics-created", base)
	created, err := service.Process(WithRequestID(context.Background(), "request-1"), item, match)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Process(context.Background(), item, match); err != nil {
		t.Fatal(err)
	}
	command := TransitionCommand{Target: IncidentResolved, Actor: "oncall", Reason: "fixed", OccurredAt: base.Add(time.Second)}
	if _, err := service.ApplyTransition(context.Background(), created.IncidentID, command); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyTransition(context.Background(), created.IncidentID, command); err != nil {
		t.Fatal(err)
	}
	reopenedItem := incidentSignal("metrics-reopened", base.Add(2*time.Second))
	if _, err := service.Process(context.Background(), reopenedItem, match); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Process(context.Background(), reopenedItem, match); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		collector prometheus.Collector
		want      float64
	}{
		"created":  {metrics.created, 1},
		"reopened": {metrics.reopened, 1},
		"resolved": {metrics.resolved, 1},
	} {
		if got := testutil.ToFloat64(test.collector); got != test.want {
			t.Errorf("%s=%v want=%v", name, got, test.want)
		}
	}
}
