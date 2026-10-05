package outbox

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	deliveries *prometheus.CounterVec
	duration   prometheus.Histogram
}

func NewMetrics(registerer prometheus.Registerer, pending func(context.Context) (int64, error)) *Metrics {
	deliveries := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "webhook_delivery_total", Help: "Webhook delivery lifecycle outcomes."}, []string{"outcome"})
	duration := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "webhook_delivery_duration_seconds", Help: "Webhook delivery attempt duration.", Buckets: prometheus.DefBuckets})
	pendingGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "outbox_pending", Help: "Outbox events pending or being delivered."}, func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		count, err := pending(ctx)
		if err != nil {
			return 0
		}
		return float64(count)
	})
	registerer.MustRegister(deliveries, duration, pendingGauge)
	return &Metrics{deliveries: deliveries, duration: duration}
}

func (m *Metrics) observe(outcome string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.deliveries.WithLabelValues(outcome).Inc()
	if elapsed >= 0 {
		m.duration.Observe(elapsed.Seconds())
	}
}

func (m *Metrics) replay() {
	if m != nil {
		m.deliveries.WithLabelValues("replay").Inc()
	}
}
