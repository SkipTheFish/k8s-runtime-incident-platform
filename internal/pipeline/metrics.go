package pipeline

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	received     *prometheus.CounterVec
	deduplicated *prometheus.CounterVec
	failed       *prometheus.CounterVec
	retries      *prometheus.CounterVec
}

func NewMetrics(registerer prometheus.Registerer, depth func() int) *Metrics {
	m := &Metrics{
		received:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "signals_received_total", Help: "Signals accepted by the queue."}, []string{"source", "type"}),
		deduplicated: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "signals_deduplicated_total", Help: "Signals rejected as duplicates."}, []string{"source", "type"}),
		failed:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "signals_failed_total", Help: "Signals sent to dead letter."}, []string{"source", "type"}),
		retries:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "event_queue_retries_total", Help: "Signal queue retries."}, []string{"source", "type"}),
	}
	registerer.MustRegister(m.received, m.deduplicated, m.failed, m.retries, prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "event_queue_depth", Help: "Current signal queue depth."}, func() float64 { return float64(depth()) }))
	return m
}

func labels(s string, t string) prometheus.Labels { return prometheus.Labels{"source": s, "type": t} }
