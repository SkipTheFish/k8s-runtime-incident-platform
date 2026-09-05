package resourcegraph

import (
	"context"
	"os"
	"testing"
	"time"

	"observability-platform/internal/database"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryUpsertEdgesReplayRebuildAndSoftDelete(t *testing.T) {
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
	if _, err := pool.Exec(context.Background(), "TRUNCATE resource_edges, runtime_resources CASCADE"); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner := ResourceTask{Action: TaskUpsert, Node: ResourceNode{Key: ResourceKey{ClusterID: "c", APIVersion: "apps/v1", Kind: "ReplicaSet", Namespace: "ns", Name: "api-abc", UID: "rs-1"}, ObservedAt: now}}
	child := ResourceTask{Action: TaskUpsert, Node: ResourceNode{Key: ResourceKey{ClusterID: "c", APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "api", UID: "pod-old"}, ObservedAt: now}, OwnerUIDs: []string{"rs-1"}}
	for _, task := range []ResourceTask{owner, child, child} {
		if err := repo.Apply(context.Background(), task); err != nil {
			t.Fatal(err)
		}
	}
	var nodes, edges int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM runtime_resources").Scan(&nodes); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM resource_edges").Scan(&edges); err != nil {
		t.Fatal(err)
	}
	if nodes != 2 || edges != 1 {
		t.Fatalf("nodes=%d edges=%d", nodes, edges)
	}
	rebuilt := child
	rebuilt.Node.Key.UID = "pod-new"
	if err := repo.Apply(context.Background(), rebuilt); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM runtime_resources WHERE name='api'").Scan(&nodes); err != nil {
		t.Fatal(err)
	}
	if nodes != 2 {
		t.Fatalf("same-name nodes=%d", nodes)
	}
	child.Action = TaskDelete
	child.Node.ObservedAt = now.Add(time.Minute)
	if err := repo.Apply(context.Background(), child); err != nil {
		t.Fatal(err)
	}
	var deleted bool
	if err := pool.QueryRow(context.Background(), "SELECT deleted_at IS NOT NULL FROM runtime_resources WHERE uid='pod-old'").Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("resource was not soft deleted")
	}
}
