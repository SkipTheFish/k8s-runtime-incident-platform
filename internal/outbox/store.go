package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxPayloadBytes = 256 << 10

type Event struct {
	EventID    string          `json:"eventID"`
	IncidentID int64           `json:"incidentID"`
	EventType  string          `json:"eventType"`
	Payload    json.RawMessage `json:"payload"`
	Attempt    int             `json:"attempt"`
}

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) ClaimBatch(ctx context.Context, limit int, lease time.Duration) ([]Event, error) {
	if limit < 1 {
		limit = 1
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET status='pending',updated_at=now() WHERE status='delivering' AND updated_at < now()-make_interval(secs => $1)`, lease.Seconds()); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `WITH due AS (
SELECT id FROM outbox_events WHERE status='pending' AND next_attempt_at <= now() ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT $1
) UPDATE outbox_events o SET status='delivering',attempt=o.attempt+1,updated_at=now() FROM due WHERE o.id=due.id
RETURNING o.event_id,o.incident_id,o.event_type,o.payload,o.attempt`, limit)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.EventID, &event.IncidentID, &event.EventType, &event.Payload, &event.Attempt); err != nil {
			rows.Close()
			return nil, err
		}
		events = append(events, event)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) MarkDelivered(ctx context.Context, eventID string) error {
	_, err := s.db.Exec(ctx, `UPDATE outbox_events SET status='delivered',delivered_at=now(),updated_at=now(),last_error='' WHERE event_id=$1 AND status='delivering'`, eventID)
	return err
}

func (s *Store) Release(ctx context.Context, eventID string, cause error) error {
	message := "delivery failed"
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 1024 {
		message = message[:1024]
	}
	_, err := s.db.Exec(ctx, `UPDATE outbox_events SET status='pending',next_attempt_at=now()+interval '1 second',updated_at=now(),last_error=$2 WHERE event_id=$1 AND status='delivering'`, eventID, message)
	return err
}

func (s *Store) PendingCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE status IN ('pending','delivering')`).Scan(&count)
	return count, err
}
