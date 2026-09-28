package incident

import (
	"context"
	"testing"
	"time"
)

func TestTransitionRejectsStaleExpectedVersion(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	service := NewTransactionService(pool, 3*time.Second)
	created, err := service.Process(context.Background(), incidentSignal("stale-version", time.Now().UTC()), incidentMatch())
	if err != nil {
		t.Fatal(err)
	}
	stale := int64(99)
	_, err = service.ApplyTransition(context.Background(), created.IncidentID, TransitionCommand{Target: IncidentAcknowledged, Actor: "oncall", Reason: "investigating", OccurredAt: time.Now().UTC(), ExpectedVersion: &stale})
	if !IsCode(err, CodeVersionConflict) {
		t.Fatalf("err=%v", err)
	}
	item, err := NewPGQueryStoreWithPool(pool).Get(context.Background(), "default", created.IncidentID)
	if err != nil || item.Version != created.Version || item.Status != string(IncidentOpen) {
		t.Fatalf("item=%+v err=%v", item, err)
	}
}
