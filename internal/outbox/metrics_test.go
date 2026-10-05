package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestOutboxMetricsTrackPendingAndLifecycleOutcomes(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry, func(context.Context) (int64, error) { return 7, nil })
	metrics.observe("success", time.Second)
	metrics.observe("retry", 2*time.Second)
	metrics.observe("dead_letter", 3*time.Second)
	metrics.replay()
	for _, outcome := range []string{"success", "retry", "dead_letter", "replay"} {
		if got := testutil.ToFloat64(metrics.deliveries.WithLabelValues(outcome)); got != 1 {
			t.Errorf("%s=%v", outcome, got)
		}
	}
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]float64{}
	for _, family := range metricFamilies {
		if family.GetName() == "outbox_pending" {
			values[family.GetName()] = family.Metric[0].GetGauge().GetValue()
		}
		if family.GetName() == "webhook_delivery_duration_seconds" {
			values[family.GetName()] = float64(family.Metric[0].GetHistogram().GetSampleCount())
		}
	}
	if values["outbox_pending"] != 7 || values["webhook_delivery_duration_seconds"] != 3 {
		t.Fatalf("values=%v", values)
	}
}
