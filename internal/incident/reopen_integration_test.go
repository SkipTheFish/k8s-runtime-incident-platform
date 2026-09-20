package incident

import (
	"context"
	"testing"
	"time"
)

func TestProcessReopensAtStoredWindowBoundaryAndIsIdempotent(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	service := NewTransactionService(pool, 3*time.Second)
	base := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	original := incidentMatch()
	original.PolicyGeneration = 3
	original.ReopenWindow = 30 * time.Minute
	created, err := service.Process(context.Background(), incidentSignal("first", base), original)
	if err != nil {
		t.Fatal(err)
	}
	resolvedAt := base.Add(10 * time.Minute)
	if _, err := service.ApplyTransition(context.Background(), created.IncidentID, TransitionCommand{Target: IncidentResolved, Actor: "oncall", Reason: "fixed", OccurredAt: resolvedAt}); err != nil {
		t.Fatal(err)
	}
	updatedPolicy := original
	updatedPolicy.PolicyGeneration = 99
	updatedPolicy.ReopenWindow = time.Minute
	reopened, err := service.Process(context.Background(), incidentSignal("boundary", resolvedAt.Add(30*time.Minute)), updatedPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Reopened || reopened.IncidentID != created.IncidentID || reopened.Version != 3 {
		t.Fatalf("reopened=%#v", reopened)
	}
	replay, err := service.Process(context.Background(), incidentSignal("boundary", resolvedAt.Add(30*time.Minute)), updatedPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Reopened || replay.Version != 3 {
		t.Fatalf("replay=%#v", replay)
	}
	var status string
	var reopenCount int
	var generation int64
	if err := pool.QueryRow(context.Background(), `SELECT status,reopen_count,policy_generation FROM incidents WHERE id=$1`, created.IncidentID).Scan(&status, &reopenCount, &generation); err != nil {
		t.Fatal(err)
	}
	if status != "reopened" || reopenCount != 1 || generation != 3 {
		t.Fatalf("status=%s reopen=%d generation=%d", status, reopenCount, generation)
	}
	var timeline, outbox int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM timeline_entries WHERE incident_id=$1 AND entry_type='incident.reopened'`, created.IncidentID).Scan(&timeline); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM outbox_events WHERE incident_id=$1 AND event_type='incident.reopened'`, created.IncidentID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if timeline != 1 || outbox != 1 {
		t.Fatalf("timeline=%d outbox=%d", timeline, outbox)
	}
}

func TestProcessCreatesNewIncidentOutsideStoredReopenWindow(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	service := NewTransactionService(pool, 3*time.Second)
	base := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	match := incidentMatch()
	match.ReopenWindow = 10 * time.Minute
	created, err := service.Process(context.Background(), incidentSignal("first-outside", base), match)
	if err != nil {
		t.Fatal(err)
	}
	resolvedAt := base.Add(time.Minute)
	if _, err := service.ApplyTransition(context.Background(), created.IncidentID, TransitionCommand{Target: IncidentResolved, Actor: "oncall", Reason: "fixed", OccurredAt: resolvedAt}); err != nil {
		t.Fatal(err)
	}
	next, err := service.Process(context.Background(), incidentSignal("outside", resolvedAt.Add(10*time.Minute+time.Nanosecond)), match)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Created || next.Reopened || next.IncidentID == created.IncidentID {
		t.Fatalf("next=%#v", next)
	}
}
