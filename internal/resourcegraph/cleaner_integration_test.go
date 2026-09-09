package resourcegraph

import (
	"context"
	"os"
	"testing"
	"time"

	"observability-platform/internal/database"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCleanExpiredHonorsBoundaryBatchAndReferences(t *testing.T) {
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
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE timeline_entries, correlation_decisions, incident_signals, incidents, dead_letter_events, processed_events, signals, resource_edges, runtime_resources RESTART IDENTITY CASCADE"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	cutoff := now.Add(-24 * time.Hour)
	insert := func(uid string, deleted time.Time) {
		t.Helper()
		if _, err := pool.Exec(context.Background(), `INSERT INTO runtime_resources(cluster_id,uid,api_version,kind,namespace,name,observed_at,deleted_at) VALUES('c',$1,'v1','Pod','ns',$1,$2,$3)`, uid, deleted, deleted); err != nil {
			t.Fatal(err)
		}
	}
	insert("old", cutoff.Add(-time.Second))
	insert("boundary", cutoff)
	insert("fresh", cutoff.Add(time.Second))
	insert("incident-ref", cutoff.Add(-time.Hour))
	insert("evidence-ref", cutoff.Add(-time.Hour))
	if _, err := pool.Exec(context.Background(), `INSERT INTO incidents(tenant_id,title,status,namespace,policy_name,started_at,updated_at,last_event_at,fingerprint,root_resource) VALUES('t','i','resolved','','p',now(),now(),now(),'cleanup-test',$1)`, map[string]any{"uid": "incident-ref"}); err != nil {
		t.Fatal(err)
	}
	var incidentID int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM incidents WHERE fingerprint='cleanup-test'`).Scan(&incidentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO timeline_entries(incident_id,idempotency_key,entry_type,message,evidence,occurred_at) VALUES($1,'cleanup-evidence','test','test',$2,now())`, incidentID, map[string]any{"resourceUID": "evidence-ref"}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	deleted, err := repo.CleanExpired(context.Background(), cutoff, 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("first batch deleted=%d", deleted)
	}
	deleted, err = repo.CleanExpired(context.Background(), cutoff, 10, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("second batch deleted=%d", deleted)
	}
	var remaining []string
	rows, err := pool.Query(context.Background(), `SELECT uid FROM runtime_resources ORDER BY uid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			t.Fatal(err)
		}
		remaining = append(remaining, uid)
	}
	want := map[string]bool{"fresh": true, "incident-ref": true, "evidence-ref": true}
	if len(remaining) != len(want) {
		t.Fatalf("remaining=%v", remaining)
	}
	for _, uid := range remaining {
		if !want[uid] {
			t.Fatalf("unexpected remaining uid %q", uid)
		}
	}
}

func TestCleanExpiredHonorsCanceledContext(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewRepository(pool).CleanExpired(ctx, time.Now(), 1, time.Second); err == nil {
		t.Fatal("expected cancellation error")
	}
}
