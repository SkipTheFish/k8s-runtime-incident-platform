package incident

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	created  prometheus.Counter
	reopened prometheus.Counter
	resolved prometheus.Counter
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	metrics := &Metrics{
		created:  prometheus.NewCounter(prometheus.CounterOpts{Name: "incidents_created_total", Help: "Incidents created after successful correlation transactions."}),
		reopened: prometheus.NewCounter(prometheus.CounterOpts{Name: "incidents_reopened_total", Help: "Resolved incidents reopened inside their policy window."}),
		resolved: prometheus.NewCounter(prometheus.CounterOpts{Name: "incidents_resolved_total", Help: "Incidents successfully transitioned to resolved."}),
	}
	registerer.MustRegister(metrics.created, metrics.reopened, metrics.resolved)
	return metrics
}
