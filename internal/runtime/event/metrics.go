package event

import "sync/atomic"

type Metrics struct {
	published uint64

	dropped uint64

	deduped uint64

	rateLimited uint64

	validationFailed uint64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncPublished() {
	atomic.AddUint64(&m.published, 1)
}

func (m *Metrics) IncDropped() {
	atomic.AddUint64(&m.dropped, 1)
}

func (m *Metrics) IncDeduped() {
	atomic.AddUint64(&m.deduped, 1)
}

func (m *Metrics) IncRateLimited() {
	atomic.AddUint64(&m.rateLimited, 1)
}

func (m *Metrics) IncValidationFailed() {
	atomic.AddUint64(&m.validationFailed, 1)
}

func (m *Metrics) Snapshot() map[string]uint64 {

	return map[string]uint64{
		"published":         atomic.LoadUint64(&m.published),
		"dropped":           atomic.LoadUint64(&m.dropped),
		"deduped":           atomic.LoadUint64(&m.deduped),
		"rateLimited":       atomic.LoadUint64(&m.rateLimited),
		"validationFailed":  atomic.LoadUint64(&m.validationFailed),
	}
}