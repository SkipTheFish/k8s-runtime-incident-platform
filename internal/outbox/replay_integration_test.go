package outbox

import (
	"context"
	"testing"
)

func TestDeadLetterReplayIsAuditedIdempotentAndKeepsEvent(t *testing.T) {
	pool := outboxTestPool(t)
	defer pool.Close()
	seedOutbox(t, pool, "dead-event")
	deliverer := &recordingDeliverer{err: classifiedFailure{retryable: false}}
	dispatcher := NewDispatcher(NewStore(pool), deliverer, Config{MaxAttempts: 5, TargetName: "agent"})
	if _, err := dispatcher.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	items, total, err := store.ListDeadLetters(context.Background(), 10, 0)
	if err != nil || total != 1 || len(items) != 1 || items[0].EventID != "dead-event" {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	first, err := store.Replay(context.Background(), "dead-event", "operator", "endpoint fixed")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Replay(context.Background(), "dead-event", "operator", "endpoint fixed")
	if err != nil || first.ReplayedAt != second.ReplayedAt || first.PreviousAttempt != 1 {
		t.Fatalf("first=%+v second=%+v err=%v", first, second, err)
	}
	var events, audits int
	if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM outbox_events WHERE event_id='dead-event'),(SELECT count(*) FROM outbox_replays WHERE event_id='dead-event')`).Scan(&events, &audits); err != nil {
		t.Fatal(err)
	}
	if events != 1 || audits != 1 {
		t.Fatalf("events=%d audits=%d", events, audits)
	}
	deliverer.err = nil
	if _, err := dispatcher.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var status string
	var attempt int
	if err := pool.QueryRow(context.Background(), `SELECT status,attempt FROM outbox_events WHERE event_id='dead-event'`).Scan(&status, &attempt); err != nil {
		t.Fatal(err)
	}
	if status != "delivered" || attempt != 2 {
		t.Fatalf("status=%s attempt=%d", status, attempt)
	}
}
