package pipeline

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
}
