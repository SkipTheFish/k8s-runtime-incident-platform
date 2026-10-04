package outbox

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrEventNotFound = errors.New("outbox event not found")
	ErrNotDeadLetter = errors.New("outbox event is not a dead letter")
)

type DeadLetter struct {
	EventID    string    `json:"eventID"`
	IncidentID int64     `json:"incidentID"`
	EventType  string    `json:"eventType"`
	Attempt    int       `json:"attempt"`
	LastError  string    `json:"lastError"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ReplayAudit struct {
	EventID         string    `json:"eventID"`
	PreviousAttempt int       `json:"previousAttempt"`
	Actor           string    `json:"actor"`
	Reason          string    `json:"reason"`
	ReplayedAt      time.Time `json:"replayedAt"`
}

func (s *Store) ListDeadLetters(ctx context.Context, limit, offset int) ([]DeadLetter, int64, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE status='dead_letter'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `SELECT event_id,incident_id,event_type,attempt,last_error,updated_at FROM outbox_events WHERE status='dead_letter' ORDER BY updated_at DESC,event_id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]DeadLetter, 0, limit)
	for rows.Next() {
		var item DeadLetter
		if err := rows.Scan(&item.EventID, &item.IncidentID, &item.EventType, &item.Attempt, &item.LastError, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) Replay(ctx context.Context, eventID, actor, reason string) (ReplayAudit, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReplayAudit{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	var attempt int
	err = tx.QueryRow(ctx, `SELECT status,attempt FROM outbox_events WHERE event_id=$1 FOR UPDATE`, eventID).Scan(&status, &attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReplayAudit{}, ErrEventNotFound
	}
	if err != nil {
		return ReplayAudit{}, err
	}
	var audit ReplayAudit
	if status == "pending" {
		err = tx.QueryRow(ctx, `SELECT event_id,previous_attempt,actor,reason,replayed_at FROM outbox_replays WHERE event_id=$1 AND previous_attempt=$2`, eventID, attempt).Scan(&audit.EventID, &audit.PreviousAttempt, &audit.Actor, &audit.Reason, &audit.ReplayedAt)
		if err == nil {
			return audit, tx.Commit(ctx)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return ReplayAudit{}, err
		}
	}
	if status != "dead_letter" {
		return ReplayAudit{}, ErrNotDeadLetter
	}
	err = tx.QueryRow(ctx, `INSERT INTO outbox_replays(event_id,previous_attempt,actor,reason) VALUES($1,$2,$3,$4) ON CONFLICT(event_id,previous_attempt) DO UPDATE SET event_id=EXCLUDED.event_id RETURNING event_id,previous_attempt,actor,reason,replayed_at`, eventID, attempt, actor, reason).Scan(&audit.EventID, &audit.PreviousAttempt, &audit.Actor, &audit.Reason, &audit.ReplayedAt)
	if err != nil {
		return ReplayAudit{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET status='pending',next_attempt_at=now(),updated_at=now(),last_error='' WHERE event_id=$1`, eventID); err != nil {
		return ReplayAudit{}, err
	}
	return audit, tx.Commit(ctx)
}
