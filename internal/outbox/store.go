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

func (s *Store) RecordAttempt(ctx context.Context, event Event, targetName, deliveryStatus string, responseStatus int, summary string, next time.Time, deadLetter bool) error {
	if len(summary) > 1024 {
		summary = summary[:1024]
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var response any
	if responseStatus > 0 {
		response = responseStatus
	}
	var nextValue any
	if !next.IsZero() {
		nextValue = next
	}
	if deadLetter {
		deliveryStatus = "failed"
	}
	_, err = tx.Exec(ctx, `INSERT INTO webhook_deliveries(event_id,attempt,target_name,status,response_status,error_summary,next_attempt_at,finished_at) VALUES($1,$2,$3,$4,$5,$6,$7,now()) ON CONFLICT(event_id,attempt) DO NOTHING`, event.EventID, event.Attempt, targetName, deliveryStatus, response, summary, nextValue)
	if err != nil {
		return err
	}
	if deliveryStatus == "succeeded" {
		_, err = tx.Exec(ctx, `UPDATE outbox_events SET status='delivered',delivered_at=now(),updated_at=now(),last_error='' WHERE event_id=$1 AND status='delivering'`, event.EventID)
	} else if deadLetter {
		_, err = tx.Exec(ctx, `UPDATE outbox_events SET status='dead_letter',updated_at=now(),last_error=$2 WHERE event_id=$1 AND status='delivering'`, event.EventID, summary)
	} else {
		_, err = tx.Exec(ctx, `UPDATE outbox_events SET status='pending',next_attempt_at=$2,updated_at=now(),last_error=$3 WHERE event_id=$1 AND status='delivering'`, event.EventID, next, summary)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
