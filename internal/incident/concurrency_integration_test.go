package incident

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestConcurrentFingerprintIdempotency50(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)

	const workers = 50
	service := NewTransactionService(pool, 10*time.Second)
	match := incidentMatch()
	started := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	var injected atomic.Bool
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-started
			item := incidentSignal(fmt.Sprintf("concurrent-%02d", index), time.Now().UTC())
			operation := func(ctx context.Context) (ProcessResult, error) {
				if index == 0 && injected.CompareAndSwap(false, true) {
					return ProcessResult{}, &pgconn.PgError{Code: "40001", Message: "injected serialization failure"}
				}
				return service.Process(ctx, item, match)
			}
			_, err := RetryTransaction(context.Background(), RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}, operation)
			errs <- err
		}(i)
	}
	close(started)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent processing: %v", err)
		}
	}
	if !injected.Load() {
		t.Fatal("transient failure was not injected")
	}

	var incidents, signals, links, timeline, decisions, outbox int
	var version int64
	err := pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM incidents WHERE fingerprint=$1 AND lower(status) IN ('open','acknowledged','reopened')),
		(SELECT count(*) FROM signals),
		(SELECT count(*) FROM incident_signals),
		(SELECT count(*) FROM timeline_entries),
		(SELECT count(*) FROM correlation_decisions),
		(SELECT count(*) FROM outbox_events),
		(SELECT version FROM incidents WHERE fingerprint=$1 AND lower(status) IN ('open','acknowledged','reopened'))`, match.Fingerprint).
		Scan(&incidents, &signals, &links, &timeline, &decisions, &outbox, &version)
	if err != nil {
		t.Fatal(err)
	}
	if incidents != 1 || signals != workers || links != workers || timeline != workers || decisions != workers || outbox != workers || version != workers {
		t.Fatalf("incidents=%d signals=%d links=%d timeline=%d decisions=%d outbox=%d version=%d", incidents, signals, links, timeline, decisions, outbox, version)
	}
}
