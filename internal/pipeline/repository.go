package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"observability-platform/internal/correlation"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDuplicate = errors.New("signal already processed")

type Repository struct{ db *pgxpool.Pool }

type DeadLetter struct {
	ID                                         int64
	SignalID, Status, ErrorCategory, LastError string
	Attempts                                   int
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Persist(ctx context.Context, s signal.Signal) error {
	owners, _ := json.Marshal(s.OwnerReferences)
	labels, _ := json.Marshal(s.Labels)
	raw := s.RawReference
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `INSERT INTO signals
(id,cluster_id,source,type,source_event_key,reason,namespace,resource_api_version,resource_kind,resource_name,resource_uid,owner_references,labels,message,raw_reference,occurred_at,observed_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) ON CONFLICT DO NOTHING`,
		s.ID, s.ClusterID, s.Source, s.Type, s.SourceEventKey, s.Reason, s.Namespace, s.Resource.APIVersion, s.Resource.Kind, s.Resource.Name, s.Resource.UID, owners, labels, s.Message, raw, s.OccurredAt, s.ObservedAt)
	if err != nil {
		return fmt.Errorf("insert signal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrDuplicate
	}
	if _, err = tx.Exec(ctx, `INSERT INTO processed_events(source,cluster_id,source_event_key,signal_id) VALUES($1,$2,$3,$4)`, s.Source, s.ClusterID, s.SourceEventKey, s.ID); err != nil {
		return fmt.Errorf("mark event processed: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *Repository) RecordDeadLetter(ctx context.Context, s signal.Signal, category string, cause error, attempts int) error {
	payload, _ := json.Marshal(s)
	summary := cause.Error()
	if len(summary) > 1024 {
		summary = summary[:1024]
	}
	_, err := r.db.Exec(ctx, `INSERT INTO dead_letter_events(signal_id,source,source_event_key,payload,error_category,last_error,attempts) VALUES($1,$2,$3,$4,$5,$6,$7)`, s.ID, s.Source, s.SourceEventKey, payload, category, summary, attempts)
	return err
}

func (r *Repository) MarkMatched(ctx context.Context, signalID string, root resourcegraph.ResourceKey) error {
	_, err := r.db.Exec(ctx, `UPDATE signals SET resolution_status='matched',root_resource_uid=$2,root_resource_kind=$3,root_resource_namespace=$4,root_resource_name=$5,resolution_error=NULL WHERE id=$1`, signalID, root.UID, root.Kind, root.Namespace, root.Name)
	return err
}

func (r *Repository) MarkUnmatched(ctx context.Context, signalID, reason string) error {
	if len(reason) > 1024 {
		reason = reason[:1024]
	}
	_, err := r.db.Exec(ctx, `UPDATE signals SET resolution_status='unmatched',resolution_error=$2 WHERE id=$1`, signalID, reason)
	return err
}

func (r *Repository) RecordDecision(ctx context.Context, item signal.Signal, result correlation.MatchResult) error {
	root, _ := json.Marshal(result.Root)
	conditions, _ := json.Marshal(result.Explain.MatchedConditions)
	ruleName, ruleVersion := result.RuleName, result.RuleVersion
	if ruleName == "" {
		ruleName = "none"
	}
	if ruleVersion == "" {
		ruleVersion = "v1"
	}
	windowStart := item.OccurredAt
	if result.Window > 0 {
		windowStart = item.OccurredAt.Truncate(result.Window)
	}
	windowEnd := windowStart.Add(result.Window)
	if !windowEnd.After(windowStart) {
		windowEnd = windowStart.Add(time.Nanosecond)
	}
	_, err := r.db.Exec(ctx, `INSERT INTO correlation_decisions(signal_id,rule_name,rule_version,policy_generation,root_resource,fingerprint,window_start,window_end,matched_conditions,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT(signal_id,rule_name,rule_version,policy_generation) DO UPDATE SET root_resource=EXCLUDED.root_resource,fingerprint=EXCLUDED.fingerprint,window_start=EXCLUDED.window_start,window_end=EXCLUDED.window_end,matched_conditions=EXCLUDED.matched_conditions,reason=EXCLUDED.reason`, item.ID, ruleName, ruleVersion, result.PolicyGeneration, root, result.Fingerprint, windowStart, windowEnd, conditions, result.Explain.Reason)
	return err
}

func (r *Repository) LoadSimulationInputs(ctx context.Context, ids []string, limit int) ([]correlation.SimulationInput, error) {
	if len(ids) > limit {
		return nil, correlation.ErrSimulationLimit
	}
	rows, err := r.db.Query(ctx, `SELECT id,cluster_id,source,type,source_event_key,reason,namespace,resource_api_version,resource_kind,resource_name,resource_uid,labels,message,occurred_at,observed_at,COALESCE(root_resource_uid,''),COALESCE(root_resource_kind,''),COALESCE(root_resource_namespace,''),COALESCE(root_resource_name,'') FROM signals WHERE id=ANY($1) LIMIT $2`, ids, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]correlation.SimulationInput, 0, len(ids))
	for rows.Next() {
		var input correlation.SimulationInput
		var labels []byte
		if err := rows.Scan(&input.Signal.ID, &input.Signal.ClusterID, &input.Signal.Source, &input.Signal.Type, &input.Signal.SourceEventKey, &input.Signal.Reason, &input.Signal.Namespace, &input.Signal.Resource.APIVersion, &input.Signal.Resource.Kind, &input.Signal.Resource.Name, &input.Signal.Resource.UID, &labels, &input.Signal.Message, &input.Signal.OccurredAt, &input.Signal.ObservedAt, &input.Root.UID, &input.Root.Kind, &input.Root.Namespace, &input.Root.Name); err != nil {
			return nil, err
		}
		input.Root.ClusterID = input.Signal.ClusterID
		_ = json.Unmarshal(labels, &input.Signal.Labels)
		result = append(result, input)
	}
	return result, rows.Err()
}

func (r *Repository) ListDeadLetters(ctx context.Context, status string, limit int) ([]DeadLetter, error) {
	rows, err := r.db.Query(ctx, `SELECT id,COALESCE(signal_id,''),status,error_category,last_error,attempts FROM dead_letter_events WHERE status=$1 ORDER BY id DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DeadLetter
	for rows.Next() {
		var item DeadLetter
		if err := rows.Scan(&item.ID, &item.SignalID, &item.Status, &item.ErrorCategory, &item.LastError, &item.Attempts); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
