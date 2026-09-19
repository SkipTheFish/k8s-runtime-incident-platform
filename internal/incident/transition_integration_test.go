package incident

import (
	"context"
	"testing"
	"time"
)

func TestAcknowledgeAndResolveTransactionsAreAuditedAndIdempotent(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	service := NewTransactionService(pool, 3*time.Second)
	created, err := service.Process(context.Background(), incidentSignal("transition-signal", time.Now().Add(-time.Hour)), incidentMatch())
	if err != nil {
		t.Fatal(err)
	}
	ack := TransitionCommand{Target: IncidentAcknowledged, Actor: "oncall", Reason: "investigating", OccurredAt: time.Now().UTC().Add(-time.Minute)}
	acknowledged, err := service.ApplyTransition(context.Background(), created.IncidentID, ack)
	if err != nil {
		t.Fatal(err)
	}
	if !acknowledged.Changed || acknowledged.Version != 2 {
		t.Fatalf("ack=%#v", acknowledged)
	}
	replay, err := service.ApplyTransition(context.Background(), created.IncidentID, ack)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Changed || replay.Version != 2 {
		t.Fatalf("ack replay=%#v", replay)
	}
	resolve := TransitionCommand{Target: IncidentResolved, Actor: "oncall", Reason: "deployment fixed", OccurredAt: time.Now().UTC()}
	resolved, err := service.ApplyTransition(context.Background(), created.IncidentID, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Changed || resolved.Version != 3 {
		t.Fatalf("resolved=%#v", resolved)
	}
	replay, err = service.ApplyTransition(context.Background(), created.IncidentID, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Changed || replay.Version != 3 {
		t.Fatalf("resolve replay=%#v", replay)
	}
	var status string
	var version, timeline, outbox int
	var actor, reason string
	if err := pool.QueryRow(context.Background(), `SELECT status,version FROM incidents WHERE id=$1`, created.IncidentID).Scan(&status, &version); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM timeline_entries WHERE incident_id=$1`, created.IncidentID).Scan(&timeline); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM outbox_events WHERE incident_id=$1`, created.IncidentID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT actor,message FROM timeline_entries WHERE incident_id=$1 AND entry_type='incident.resolved'`, created.IncidentID).Scan(&actor, &reason); err != nil {
		t.Fatal(err)
	}
	if status != "resolved" || version != 3 || timeline != 3 || outbox != 3 || actor != "oncall" || reason != "deployment fixed" {
		t.Fatalf("status=%s version=%d timeline=%d outbox=%d actor=%s reason=%s", status, version, timeline, outbox, actor, reason)
	}
}

func TestTransitionTransactionRejectsIllegalState(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	service := NewTransactionService(pool, 3*time.Second)
	created, err := service.Process(context.Background(), incidentSignal("illegal", time.Now()), incidentMatch())
	if err != nil {
		t.Fatal(err)
	}
	resolve := TransitionCommand{Target: IncidentResolved, Actor: "oncall", Reason: "done", OccurredAt: time.Now()}
	if _, err := service.ApplyTransition(context.Background(), created.IncidentID, resolve); err != nil {
		t.Fatal(err)
	}
	_, err = service.ApplyTransition(context.Background(), created.IncidentID, TransitionCommand{Target: IncidentAcknowledged, Actor: "oncall", Reason: "late", OccurredAt: time.Now()})
	if !IsCode(err, CodeInvalidTransition) {
		t.Fatalf("error=%v", err)
	}
}
