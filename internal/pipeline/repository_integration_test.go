package pipeline

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"observability-platform/internal/correlation"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

func TestRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(ctx, "TRUNCATE dead_letter_events, processed_events, signals CASCADE"); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	s := signal.Normalize(signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Type: signal.TypeWarning, SourceEventKey: "e", Reason: "BackOff", Resource: signal.ObjectReference{UID: "u"}, OccurredAt: time.Now(), ObservedAt: time.Now()})
	for i := 0; i < 100; i++ {
		err := repo.Persist(ctx, s)
		if i == 0 && err != nil {
			t.Fatal(err)
		}
		if i > 0 && !errors.Is(err, ErrDuplicate) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	var count int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM signals").Scan(&count); err != nil || count != 1 {
		t.Fatalf("signal count=%d err=%v", count, err)
	}
	if err := repo.RecordDeadLetter(ctx, s, "permanent", errors.New("permanent failure"), 6); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListDeadLetters(ctx, "pending", 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("dead letters=%d err=%v", len(items), err)
	}
	decision := correlation.MatchResult{Matched: true, RuleName: "crash-loop-backoff", RuleVersion: "v1", PolicyGeneration: 4, Root: resourcegraph.ResourceKey{ClusterID: "c", Kind: "Deployment", UID: "root"}, Fingerprint: "fingerprint", Window: 10 * time.Minute, Explain: correlation.Explain{MatchedConditions: []string{"reason matched"}, Reason: "deterministic rule match"}}
	if err := repo.RecordDecision(ctx, s, decision); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordDecision(ctx, s, decision); err != nil {
		t.Fatal(err)
	}
	var rule, reason string
	var generation int64
	if err := db.QueryRow(ctx, `SELECT rule_name,policy_generation,reason FROM correlation_decisions WHERE signal_id=$1`, s.ID).Scan(&rule, &generation, &reason); err != nil {
		t.Fatal(err)
	}
	if rule != "crash-loop-backoff" || generation != 4 || reason != "deterministic rule match" {
		t.Fatalf("decision=%s %d %s", rule, generation, reason)
	}
	if err := repo.MarkMatched(ctx, s.ID, resourcegraph.ResourceKey{ClusterID: "c", Kind: "Deployment", UID: "root"}); err != nil {
		t.Fatal(err)
	}
	var before, after int
	if err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM incidents)+(SELECT count(*) FROM timeline_entries)+(SELECT count(*) FROM correlation_decisions)+(SELECT count(*) FROM outbox_events)`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	simulator := correlation.NewSimulator(correlation.NewRegistry(correlation.NewCrashLoopBackOffRule(1)), repo, 10)
	result, err := simulator.Simulate(ctx, "crash-loop-backoff", correlation.SimulationRequest{Generation: 4, HistoricalSignalIDs: []string{s.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if result.InputCount != 1 || len(result.Incidents) != 1 {
		t.Fatalf("simulation=%#v", result)
	}
	if err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM incidents)+(SELECT count(*) FROM timeline_entries)+(SELECT count(*) FROM correlation_decisions)+(SELECT count(*) FROM outbox_events)`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("business rows changed: before=%d after=%d", before, after)
	}
}
