package pipeline

import "github.com/prometheus/client_golang/prometheus"

type CorrelationMetrics struct {
	duration  prometheus.Observer
	unmatched prometheus.Counter
	conflicts prometheus.Counter
}

func NewCorrelationMetrics(registerer prometheus.Registerer) *CorrelationMetrics {
	duration := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "correlation_duration_seconds", Help: "Time spent evaluating correlation rules.", Buckets: prometheus.DefBuckets})
	unmatched := prometheus.NewCounter(prometheus.CounterOpts{Name: "signals_unmatched_total", Help: "Signals retained without a matching correlation rule."})
	conflicts := prometheus.NewCounter(prometheus.CounterOpts{Name: "correlation_policy_conflicts_total", Help: "Signals blocked by same-priority correlation rule conflicts."})
	registerer.MustRegister(duration, unmatched, conflicts)
	return &CorrelationMetrics{duration: duration, unmatched: unmatched, conflicts: conflicts}
}
