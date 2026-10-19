package outbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"observability-platform/internal/database"

	"github.com/jackc/pgx/v5/pgxpool"
)

type recordingDeliverer struct {
	mu     sync.Mutex
	events []Event
	err    error
}

func (d *recordingDeliverer) Deliver(_ context.Context, event Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.events = append(d.events, event)
	return d.err
}

func (d *recordingDeliverer) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.events)
}

func TestClaimRecoversAfterRestartAndKeepsEventID(t *testing.T) {
	pool := outboxTestPool(t)
	defer pool.Close()
	seedOutbox(t, pool, "stable-event")
	store := NewStore(pool)
	claimed, err := store.ClaimBatch(context.Background(), 1, time.Millisecond)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE outbox_events SET updated_at=now()-interval '1 minute' WHERE event_id='stable-event'`); err != nil {
		t.Fatal(err)
	}
	restarted := NewStore(pool)
	claimedAgain, err := restarted.ClaimBatch(context.Background(), 1, time.Second)
	if err != nil || len(claimedAgain) != 1 || claimedAgain[0].EventID != claimed[0].EventID || claimedAgain[0].Attempt != 2 {
		t.Fatalf("claimedAgain=%+v err=%v", claimedAgain, err)
	}
}

func TestDispatcherDeliversDueEventAndReleasesFailure(t *testing.T) {
	pool := outboxTestPool(t)
	defer pool.Close()
	seedOutbox(t, pool, "delivery-event")
	deliverer := &recordingDeliverer{}
	dispatcher := NewDispatcher(NewStore(pool), deliverer, Config{BatchSize: 10})
	count, err := dispatcher.ProcessOnce(context.Background())
	if err != nil || count != 1 || deliverer.count() != 1 {
		t.Fatalf("count=%d events=%d err=%v", count, deliverer.count(), err)
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM outbox_events WHERE event_id='delivery-event'`).Scan(&status); err != nil || status != "delivered" {
		t.Fatalf("status=%s err=%v", status, err)
	}

	seedOutbox(t, pool, "failed-event")
	deliverer.err = errors.New("temporary")
	if _, err := dispatcher.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var eventID string
	if err := pool.QueryRow(context.Background(), `SELECT event_id FROM outbox_events WHERE event_id='failed-event' AND status='pending'`).Scan(&eventID); err != nil || eventID != "failed-event" {
		t.Fatalf("eventID=%s err=%v", eventID, err)
	}
}

func TestDispatcherConcurrentClaimsDeliverEachEventOnce(t *testing.T) {
	pool := outboxTestPool(t)
	defer pool.Close()
	seedOutboxBatch(t, pool, 40)
	deliverer := &recordingDeliverer{}
	store := NewStore(pool)
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			dispatcher := NewDispatcher(store, deliverer, Config{BatchSize: 5})
			if _, err := dispatcher.ProcessOnce(context.Background()); err != nil {
				t.Errorf("process: %v", err)
			}
		}()
	}
	group.Wait()
	if got := deliverer.count(); got != 40 {
		t.Fatalf("deliveries=%d want=40", got)
	}
	var delivered int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM outbox_events WHERE status='delivered'`).Scan(&delivered); err != nil || delivered != 40 {
		t.Fatalf("delivered=%d err=%v", delivered, err)
	}
}

func TestTraceLinkExtractsOnlyValidTraceID(t *testing.T) {
	want := "4bf92f3577b34da6a3ce929d0e0e4736"
	got := traceLink([]byte(`{"traceID":"` + want + `","spanID":"00f067aa0ba902b7"}`))
	if !got.IsValid() || got.TraceID().String() != want || !got.IsRemote() {
		t.Fatalf("span context=%v", got)
	}
	for _, payload := range [][]byte{nil, []byte(`{}`), []byte(`{"traceID":"invalid"}`), []byte(`not-json`)} {
		if traceLink(payload).IsValid() {
			t.Fatalf("invalid payload produced trace link: %s", payload)
		}
	}
}

func seedOutboxBatch(t *testing.T, pool *pgxpool.Pool, count int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE webhook_deliveries,outbox_events,timeline_entries,correlation_decisions,incident_signals,incidents,dead_letter_events,processed_events,signals RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < count; index++ {
		eventID := fmt.Sprintf("concurrent-%d", index)
		var incidentID int64
		if err := pool.QueryRow(context.Background(), `INSERT INTO incidents(tenant_id,title,status,namespace,policy_name,severity,started_at,updated_at,last_event_at,fingerprint,category,root_resource,version,correlation_policy,policy_generation,reopen_window_seconds) VALUES('default','test','open','','','warning',now(),now(),now(),$1,'test','{}',1,'test',1,1800) RETURNING id`, "outbox-"+eventID).Scan(&incidentID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), `INSERT INTO outbox_events(event_id,incident_id,event_type,payload) VALUES($1,$2,'incident.created','{"ok":true}')`, eventID, incidentID); err != nil {
			t.Fatal(err)
		}
	}
}

func outboxTestPool(t *testing.T) *pgxpool.Pool {
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

func seedOutbox(t *testing.T, pool *pgxpool.Pool, eventID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE webhook_deliveries,outbox_events,timeline_entries,correlation_decisions,incident_signals,incidents,dead_letter_events,processed_events,signals RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	var incidentID int64
	err := pool.QueryRow(context.Background(), `INSERT INTO incidents(tenant_id,title,status,namespace,policy_name,severity,started_at,updated_at,last_event_at,fingerprint,category,root_resource,version,correlation_policy,policy_generation,reopen_window_seconds) VALUES('default','test','open','','','warning',now(),now(),now(),$1,'test','{}',1,'test',1,1800) RETURNING id`, "outbox-"+eventID).Scan(&incidentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO outbox_events(event_id,incident_id,event_type,payload) VALUES($1,$2,'incident.created','{"ok":true}')`, eventID, incidentID); err != nil {
		t.Fatal(err)
	}
}
