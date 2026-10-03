package outbox

import (
	"context"
	"testing"
	"time"
)

type classifiedFailure struct {
	retryable  bool
	retryAfter time.Duration
}

func (e classifiedFailure) Error() string             { return "temporary token=redacted-by-client" }
func (e classifiedFailure) Retryable() bool           { return e.retryable }
func (e classifiedFailure) HTTPStatus() int           { return 503 }
func (e classifiedFailure) ResponseSummary() string   { return "bounded summary" }
func (e classifiedFailure) RetryAfter() time.Duration { return e.retryAfter }

func TestDispatcherRecordsAttemptAndSchedulesWithoutSleeping(t *testing.T) {
	pool := outboxTestPool(t)
	defer pool.Close()
	seedOutbox(t, pool, "retry-event")
	deliverer := &recordingDeliverer{err: classifiedFailure{retryable: true}}
	dispatcher := NewDispatcher(NewStore(pool), deliverer, Config{BaseDelay: 10 * time.Second, MaxDelay: time.Minute, MaxAttempts: 3, TargetName: "agent"})
	fixed := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	dispatcher.now = func() time.Time { return fixed }
	dispatcher.random = func() float64 { return 0 }
	if _, err := dispatcher.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var status, target, summary string
	var next time.Time
	if err := pool.QueryRow(context.Background(), `SELECT status,target_name,error_summary,next_attempt_at FROM webhook_deliveries WHERE event_id='retry-event' AND attempt=1`).Scan(&status, &target, &summary, &next); err != nil {
		t.Fatal(err)
	}
	if status != "retryable" || target != "agent" || summary != "bounded summary" || !next.Equal(fixed.Add(5*time.Second)) {
		t.Fatalf("status=%s target=%s summary=%s next=%s", status, target, summary, next)
	}
}
