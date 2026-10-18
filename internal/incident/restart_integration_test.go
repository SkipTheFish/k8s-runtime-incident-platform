package incident

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"observability-platform/internal/outbox"
)

type restartDeliverer struct{ calls atomic.Int32 }

func (d *restartDeliverer) Deliver(context.Context, outbox.Event) error {
	d.calls.Add(1)
	return nil
}

func TestRestartRetainsIncidentReplayAndResumesOutbox(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	item := incidentSignal("restart-signal", time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC))
	match := incidentMatch()
	beforeRestart, err := NewTransactionService(pool, 3*time.Second).Process(context.Background(), item, match)
	if err != nil || !beforeRestart.Created {
		t.Fatalf("before restart=%#v err=%v", beforeRestart, err)
	}

	// New service/store/dispatcher values model a process restart over the same PostgreSQL state.
	afterRestart, err := NewTransactionService(pool, 3*time.Second).Process(context.Background(), item, match)
	if err != nil || afterRestart.Attached || afterRestart.IncidentID != beforeRestart.IncidentID || afterRestart.Version != beforeRestart.Version {
		t.Fatalf("after restart=%#v err=%v", afterRestart, err)
	}
	var incidents, timeline, decisions, signals int
	if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM incidents),(SELECT count(*) FROM timeline_entries),(SELECT count(*) FROM correlation_decisions),(SELECT count(*) FROM signals)`).Scan(&incidents, &timeline, &decisions, &signals); err != nil {
		t.Fatal(err)
	}
	if incidents != 1 || timeline != 1 || decisions != 1 || signals != 1 {
		t.Fatalf("incidents=%d timeline=%d decisions=%d signals=%d", incidents, timeline, decisions, signals)
	}

	deliverer := &restartDeliverer{}
	dispatcher := outbox.NewDispatcher(outbox.NewStore(pool), deliverer, outbox.Config{BatchSize: 10})
	count, err := dispatcher.ProcessOnce(context.Background())
	if err != nil || count != 1 || deliverer.calls.Load() != 1 {
		t.Fatalf("dispatch count=%d calls=%d err=%v", count, deliverer.calls.Load(), err)
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM outbox_events WHERE incident_id=$1`, beforeRestart.IncidentID).Scan(&status); err != nil || status != "delivered" {
		t.Fatalf("outbox status=%s err=%v", status, err)
	}
}
