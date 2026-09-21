package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"observability-platform/internal/correlation"
	"observability-platform/internal/signal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProcessResult struct {
	IncidentID int64
	Version    int64
	Created    bool
	Attached   bool
	Reopened   bool
}

type TransitionOutcome struct {
	IncidentID int64
	Status     IncidentStatus
	Version    int64
	Changed    bool
}

func (s *TransactionService) ApplyTransition(ctx context.Context, incidentID int64, command TransitionCommand) (TransitionOutcome, error) {
	return RetryTransaction(ctx, s.retry, func(ctx context.Context) (TransitionOutcome, error) {
		return s.applyTransitionOnce(ctx, incidentID, command)
	})
}

func (s *TransactionService) applyTransitionOnce(ctx context.Context, incidentID int64, command TransitionCommand) (TransitionOutcome, error) {
	if command.Target != IncidentAcknowledged && command.Target != IncidentResolved {
		return TransitionOutcome{}, &DomainError{Code: CodeInvalidCommand, Message: "only acknowledge and resolve commands are supported"}
	}
	txCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	tx, err := s.db.BeginTx(txCtx, pgx.TxOptions{})
	if err != nil {
		return TransitionOutcome{}, err
	}
	defer tx.Rollback(txCtx)
	var current IncidentStatus
	var version int64
	if err := tx.QueryRow(txCtx, `SELECT status,version FROM incidents WHERE id=$1 FOR UPDATE`, incidentID).Scan(&current, &version); err != nil {
		return TransitionOutcome{}, err
	}
	transition, err := Transition(current, command)
	if err != nil {
		return TransitionOutcome{}, err
	}
	if !transition.Changed {
		if err := tx.Commit(txCtx); err != nil {
			return TransitionOutcome{}, err
		}
		return TransitionOutcome{IncidentID: incidentID, Status: current, Version: version}, nil
	}
	nextVersion := version + 1
	if command.Target == IncidentAcknowledged {
		_, err = tx.Exec(txCtx, `UPDATE incidents SET status=$2,acknowledged_at=$3,updated_at=$3,version=$4 WHERE id=$1`, incidentID, command.Target, command.OccurredAt, nextVersion)
	} else {
		_, err = tx.Exec(txCtx, `UPDATE incidents SET status=$2,resolved_at=$3,updated_at=$3,version=$4 WHERE id=$1`, incidentID, command.Target, command.OccurredAt, nextVersion)
	}
	if err != nil {
		return TransitionOutcome{}, err
	}
	eventType := "incident." + string(command.Target)
	key := fmt.Sprintf("transition:%s:v%d", command.Target, nextVersion)
	if _, err := tx.Exec(txCtx, `INSERT INTO timeline_entries(incident_id,idempotency_key,entry_type,actor,message,evidence,occurred_at) VALUES($1,$2,$3,$4,$5,'{}',$6) ON CONFLICT(incident_id,idempotency_key) DO NOTHING`, incidentID, key, eventType, command.Actor, command.Reason, command.OccurredAt); err != nil {
		return TransitionOutcome{}, err
	}
	payload, _ := json.Marshal(map[string]any{"incidentID": incidentID, "status": command.Target, "version": nextVersion, "actor": command.Actor, "reason": command.Reason, "occurredAt": command.OccurredAt})
	if _, err := tx.Exec(txCtx, `INSERT INTO outbox_events(event_id,incident_id,event_type,payload) VALUES($1,$2,$3,$4) ON CONFLICT(event_id) DO NOTHING`, fmt.Sprintf("%s:%d:v%d", eventType, incidentID, nextVersion), incidentID, eventType, payload); err != nil {
		return TransitionOutcome{}, err
	}
	if err := tx.Commit(txCtx); err != nil {
		return TransitionOutcome{}, err
	}
	return TransitionOutcome{IncidentID: incidentID, Status: command.Target, Version: nextVersion, Changed: true}, nil
}

type TransactionService struct {
	db      *pgxpool.Pool
	timeout time.Duration
	retry   RetryPolicy
}

func NewTransactionService(db *pgxpool.Pool, timeout time.Duration) *TransactionService {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &TransactionService{db: db, timeout: timeout, retry: DefaultRetryPolicy()}
}

func (s *TransactionService) WithRetryPolicy(policy RetryPolicy) *TransactionService {
	s.retry = policy
	return s
}

func (s *TransactionService) Process(ctx context.Context, item signal.Signal, match correlation.MatchResult) (ProcessResult, error) {
	return RetryTransaction(ctx, s.retry, func(ctx context.Context) (ProcessResult, error) {
		return s.processOnce(ctx, item, match)
	})
}

func (s *TransactionService) processOnce(ctx context.Context, item signal.Signal, match correlation.MatchResult) (ProcessResult, error) {
	txCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	tx, err := s.db.BeginTx(txCtx, pgx.TxOptions{})
	if err != nil {
		return ProcessResult{}, err
	}
	defer tx.Rollback(txCtx)
	if err := persistSignalTx(txCtx, tx, item); err != nil {
		return ProcessResult{}, err
	}
	var existing ProcessResult
	err = tx.QueryRow(txCtx, `SELECT i.id,i.version FROM incident_signals x JOIN incidents i ON i.id=x.incident_id WHERE x.signal_id=$1 FOR UPDATE OF i`, item.ID).Scan(&existing.IncidentID, &existing.Version)
	if err == nil {
		if err := tx.Commit(txCtx); err != nil {
			return ProcessResult{}, err
		}
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return ProcessResult{}, err
	}
	rootJSON, _ := json.Marshal(match.Root)
	occurred := item.OccurredAt
	if occurred.IsZero() {
		occurred = item.ObservedAt
	}
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	result := ProcessResult{}
	err = tx.QueryRow(txCtx, `SELECT id,version FROM incidents WHERE fingerprint=$1 AND lower(status) IN ('open','acknowledged','reopened') FOR UPDATE`, match.Fingerprint).Scan(&result.IncidentID, &result.Version)
	if err == pgx.ErrNoRows {
		var resolvedID, resolvedVersion int64
		var resolvedAt time.Time
		var reopenSeconds int64
		resolvedErr := tx.QueryRow(txCtx, `SELECT id,version,resolved_at,reopen_window_seconds FROM incidents WHERE fingerprint=$1 AND lower(status)='resolved' AND resolved_at IS NOT NULL ORDER BY resolved_at DESC,id DESC LIMIT 1 FOR UPDATE`, match.Fingerprint).Scan(&resolvedID, &resolvedVersion, &resolvedAt, &reopenSeconds)
		if resolvedErr == nil && !occurred.After(resolvedAt.Add(time.Duration(reopenSeconds)*time.Second)) {
			result.IncidentID, result.Version, result.Reopened = resolvedID, resolvedVersion+1, true
			_, err = tx.Exec(txCtx, `UPDATE incidents SET status='reopened',reopen_count=reopen_count+1,version=$2,resolved_at=NULL,updated_at=$3,last_event_at=$3 WHERE id=$1`, resolvedID, result.Version, occurred)
		} else if resolvedErr != nil && resolvedErr != pgx.ErrNoRows {
			return ProcessResult{}, resolvedErr
		} else {
			reopenSeconds = int64(match.ReopenWindow / time.Second)
			if reopenSeconds < 1 {
				reopenSeconds = 1
			}
			err = tx.QueryRow(txCtx, `INSERT INTO incidents(tenant_id,title,status,namespace,policy_name,severity,started_at,updated_at,last_event_at,fingerprint,category,root_resource,version,correlation_policy,policy_generation,reopen_window_seconds)
VALUES('default',$1,'open',$2,$3,$4,$5,$5,$5,$6,$7,$8,1,$3,$9,$10)
ON CONFLICT(fingerprint) WHERE lower(status) IN ('open','acknowledged','reopened') DO NOTHING RETURNING id,version`, incidentTitle(match, item), item.Namespace, match.RuleName, match.Severity, occurred, match.Fingerprint, match.RuleName, rootJSON, match.PolicyGeneration, reopenSeconds).Scan(&result.IncidentID, &result.Version)
			if err == pgx.ErrNoRows {
				err = tx.QueryRow(txCtx, `SELECT id,version FROM incidents WHERE fingerprint=$1 AND lower(status) IN ('open','acknowledged','reopened') FOR UPDATE`, match.Fingerprint).Scan(&result.IncidentID, &result.Version)
			} else if err == nil {
				result.Created = true
			}
		}
	}
	if err != nil {
		return ProcessResult{}, fmt.Errorf("find or create active incident: %w", err)
	}
	tag, err := tx.Exec(txCtx, `INSERT INTO incident_signals(incident_id,signal_id,attached_at) VALUES($1,$2,$3) ON CONFLICT(signal_id) DO NOTHING`, result.IncidentID, item.ID, occurred)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("attach signal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if err := tx.QueryRow(txCtx, `SELECT i.id,i.version FROM incident_signals x JOIN incidents i ON i.id=x.incident_id WHERE x.signal_id=$1`, item.ID).Scan(&result.IncidentID, &result.Version); err != nil {
			return ProcessResult{}, err
		}
		if err := tx.Commit(txCtx); err != nil {
			return ProcessResult{}, err
		}
		return result, nil
	}
	result.Attached = true
	if !result.Created && !result.Reopened {
		if err := tx.QueryRow(txCtx, `UPDATE incidents SET version=version+1,updated_at=$2,last_event_at=$2 WHERE id=$1 RETURNING version`, result.IncidentID, occurred).Scan(&result.Version); err != nil {
			return ProcessResult{}, err
		}
	}
	evidence, _ := json.Marshal(match.Evidence)
	entryType := "incident.updated"
	if result.Created {
		entryType = "incident.created"
	} else if result.Reopened {
		entryType = "incident.reopened"
	}
	if _, err := tx.Exec(txCtx, `INSERT INTO timeline_entries(incident_id,idempotency_key,entry_type,actor,message,evidence,occurred_at) VALUES($1,$2,$3,'system',$4,$5,$6) ON CONFLICT(incident_id,idempotency_key) DO NOTHING`, result.IncidentID, "signal:"+item.ID+":correlated", entryType, match.Explain.Reason, evidence, occurred); err != nil {
		return ProcessResult{}, err
	}
	if err := recordDecisionTx(txCtx, tx, result.IncidentID, item, match); err != nil {
		return ProcessResult{}, err
	}
	payload, _ := json.Marshal(map[string]any{"incidentID": result.IncidentID, "signalID": item.ID, "fingerprint": match.Fingerprint, "version": result.Version})
	if _, err := tx.Exec(txCtx, `INSERT INTO outbox_events(event_id,incident_id,event_type,payload) VALUES($1,$2,$3,$4) ON CONFLICT(event_id) DO NOTHING`, entryType+":"+item.ID, result.IncidentID, entryType, payload); err != nil {
		return ProcessResult{}, err
	}
	if _, err := tx.Exec(txCtx, `UPDATE signals SET resolution_status='matched',root_resource_uid=$2,root_resource_kind=$3,root_resource_namespace=$4,root_resource_name=$5,resolution_error=NULL WHERE id=$1`, item.ID, match.Root.UID, match.Root.Kind, match.Root.Namespace, match.Root.Name); err != nil {
		return ProcessResult{}, err
	}
	if err := tx.Commit(txCtx); err != nil {
		return ProcessResult{}, err
	}
	return result, nil
}

func persistSignalTx(ctx context.Context, tx pgx.Tx, item signal.Signal) error {
	owners, _ := json.Marshal(item.OwnerReferences)
	labels, _ := json.Marshal(item.Labels)
	raw := item.RawReference
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `INSERT INTO signals(id,cluster_id,source,type,source_event_key,reason,namespace,resource_api_version,resource_kind,resource_name,resource_uid,owner_references,labels,message,raw_reference,occurred_at,observed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) ON CONFLICT DO NOTHING`, item.ID, item.ClusterID, item.Source, item.Type, item.SourceEventKey, item.Reason, item.Namespace, item.Resource.APIVersion, item.Resource.Kind, item.Resource.Name, item.Resource.UID, owners, labels, item.Message, raw, item.OccurredAt, item.ObservedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO processed_events(source,cluster_id,source_event_key,signal_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, item.Source, item.ClusterID, item.SourceEventKey, item.ID)
	return err
}
func recordDecisionTx(ctx context.Context, tx pgx.Tx, incidentID int64, item signal.Signal, result correlation.MatchResult) error {
	root, _ := json.Marshal(result.Root)
	conditions, _ := json.Marshal(result.Explain.MatchedConditions)
	start := item.OccurredAt
	if result.Window > 0 {
		start = start.Truncate(result.Window)
	}
	end := start.Add(result.Window)
	if !end.After(start) {
		end = start.Add(time.Nanosecond)
	}
	_, err := tx.Exec(ctx, `INSERT INTO correlation_decisions(incident_id,signal_id,rule_name,rule_version,policy_generation,root_resource,fingerprint,window_start,window_end,matched_conditions,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(signal_id,rule_name,rule_version,policy_generation) DO UPDATE SET incident_id=EXCLUDED.incident_id,root_resource=EXCLUDED.root_resource,fingerprint=EXCLUDED.fingerprint,window_start=EXCLUDED.window_start,window_end=EXCLUDED.window_end,matched_conditions=EXCLUDED.matched_conditions,reason=EXCLUDED.reason`, incidentID, item.ID, result.RuleName, result.RuleVersion, result.PolicyGeneration, root, result.Fingerprint, start, end, conditions, result.Explain.Reason)
	return err
}
func incidentTitle(match correlation.MatchResult, item signal.Signal) string {
	if match.RuleName != "" {
		return match.RuleName + " on " + match.Root.Kind + "/" + match.Root.Name
	}
	return item.Reason
}
