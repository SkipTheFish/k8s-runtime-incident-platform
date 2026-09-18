package incident

import (
	"context"
	"os"
	"testing"
	"time"

	"observability-platform/internal/correlation"
	"observability-platform/internal/database"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTransactionServiceCreatesThenUpdatesConsistently(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	service := NewTransactionService(pool, 3*time.Second)
	match := incidentMatch()
	first := incidentSignal("signal-1", time.Now().UTC().Add(-time.Minute))
	second := incidentSignal("signal-2", time.Now().UTC())
	created, err := service.Process(context.Background(), first, match)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Created || !created.Attached || created.Version != 1 {
		t.Fatalf("created=%#v", created)
	}
	updated, err := service.Process(context.Background(), second, match)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Created || !updated.Attached || updated.IncidentID != created.IncidentID || updated.Version != 2 {
		t.Fatalf("updated=%#v", updated)
	}
	replayed, err := service.Process(context.Background(), second, match)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Attached || replayed.Version != 2 {
		t.Fatalf("replayed=%#v", replayed)
	}
	assertCount := func(table string, want int) {
		t.Helper()
		var got int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s count=%d want=%d", table, got, want)
		}
	}
	assertCount("incidents", 1)
	assertCount("signals", 2)
	assertCount("incident_signals", 2)
	assertCount("timeline_entries", 2)
	assertCount("correlation_decisions", 2)
	assertCount("outbox_events", 2)
}

func TestTransactionServiceCanceledContextLeavesNoPartialRows(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewTransactionService(pool, time.Second).Process(ctx, incidentSignal("cancelled", time.Now()), incidentMatch()); err == nil {
		t.Fatal("expected cancellation")
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM signals)+(SELECT count(*) FROM incidents)+(SELECT count(*) FROM outbox_events)`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("partial rows=%d", count)
	}
}

func incidentTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	db, err := database.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}
func truncateIncidentTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE webhook_deliveries,outbox_events,timeline_entries,correlation_decisions,incident_signals,incidents,dead_letter_events,processed_events,signals RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
}
func incidentSignal(id string, occurred time.Time) signal.Signal {
	return signal.Normalize(signal.Signal{ID: id, ClusterID: "c", Source: signal.SourceKubernetes, Type: signal.TypeWarning, SourceEventKey: id, Reason: "BackOff", Namespace: "apps", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "apps", Name: id, UID: id}, OccurredAt: occurred, ObservedAt: occurred})
}
func incidentMatch() correlation.MatchResult {
	return correlation.MatchResult{Matched: true, RuleName: "crash-loop-backoff", RuleVersion: "v1", PolicyGeneration: 3, Fingerprint: "fingerprint-1", Root: resourcegraph.ResourceKey{ClusterID: "c", APIVersion: "apps/v1", Kind: "Deployment", Namespace: "apps", Name: "api", UID: "deployment-1"}, Window: 10 * time.Minute, ReopenWindow: 30 * time.Minute, Severity: "critical", Evidence: []correlation.Evidence{{Kind: "signal", Summary: "restart backoff"}}, Explain: correlation.Explain{MatchedConditions: []string{"BackOff"}, Reason: "container restart backoff"}}
}
