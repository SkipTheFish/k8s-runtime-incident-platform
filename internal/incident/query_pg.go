package incident

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"observability-platform/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGQueryStore struct{ db *pgxpool.Pool }

func NewPGQueryStore() *PGQueryStore                         { return &PGQueryStore{db: repository.DB} }
func NewPGQueryStoreWithPool(db *pgxpool.Pool) *PGQueryStore { return &PGQueryStore{db: db} }

const incidentViewColumns = `id,COALESCE(title,''),status,COALESCE(severity,''),namespace,policy_name,category,fingerprint,root_resource,started_at,updated_at,resolved_at,acknowledged_at,version,policy_generation`

func (s *PGQueryStore) List(ctx context.Context, tenant string, filter ListFilter) ([]IncidentView, int64, error) {
	args := []any{tenant}
	where := `tenant_id=$1`
	add := func(clause string, value any) { args = append(args, value); where += clause }
	if filter.Status != "" {
		add(` AND lower(status)=$`+itoa(len(args)+1), filter.Status)
	}
	if filter.Severity != "" {
		add(` AND lower(COALESCE(severity,''))=$`+itoa(len(args)+1), filter.Severity)
	}
	if filter.From != nil {
		add(` AND updated_at >= $`+itoa(len(args)+1), *filter.From)
	}
	if filter.To != nil {
		add(` AND updated_at <= $`+itoa(len(args)+1), *filter.To)
	}
	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM incidents WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if filter.CursorAt != nil {
		args = append(args, *filter.CursorAt, filter.CursorID)
		where += ` AND (updated_at,id) < ($` + itoa(len(args)-1) + `,$` + itoa(len(args)) + `)`
	}
	args = append(args, filter.PageSize)
	query := `SELECT ` + incidentViewColumns + ` FROM incidents WHERE ` + where + ` ORDER BY updated_at DESC,id DESC LIMIT $` + itoa(len(args))
	if filter.CursorAt == nil {
		args = append(args, (filter.Page-1)*filter.PageSize)
		query += ` OFFSET $` + itoa(len(args))
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]IncidentView, 0, filter.PageSize)
	for rows.Next() {
		item, err := scanIncident(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *PGQueryStore) Get(ctx context.Context, tenant string, id int64) (*IncidentView, error) {
	item, err := scanIncident(s.db.QueryRow(ctx, `SELECT `+incidentViewColumns+` FROM incidents WHERE tenant_id=$1 AND id=$2`, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIncidentNotFound
	}
	return &item, err
}

func (s *PGQueryStore) Timeline(ctx context.Context, tenant string, id int64, page, pageSize int) ([]TimelineView, int64, error) {
	if _, err := s.Get(ctx, tenant, id); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM timeline_entries WHERE incident_id=$1`, id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,entry_type,actor,message,evidence,occurred_at FROM timeline_entries WHERE incident_id=$1 ORDER BY occurred_at,id LIMIT $2 OFFSET $3`, id, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]TimelineView, 0, pageSize)
	for rows.Next() {
		var item TimelineView
		var evidence []byte
		if err := rows.Scan(&item.ID, &item.Type, &item.Actor, &item.Message, &evidence, &item.OccurredAt); err != nil {
			return nil, 0, err
		}
		item.Evidence, item.Truncated = SanitizeJSON(evidence)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *PGQueryStore) Correlation(ctx context.Context, tenant string, id int64, page, pageSize int) ([]CorrelationView, int64, error) {
	if _, err := s.Get(ctx, tenant, id); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM correlation_decisions WHERE incident_id=$1`, id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,signal_id,rule_name,rule_version,policy_generation,root_resource,fingerprint,matched_conditions,reason,decided_at FROM correlation_decisions WHERE incident_id=$1 ORDER BY decided_at,id LIMIT $2 OFFSET $3`, id, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]CorrelationView, 0, pageSize)
	for rows.Next() {
		var item CorrelationView
		var root, conditions []byte
		if err := rows.Scan(&item.ID, &item.SignalID, &item.RuleName, &item.RuleVersion, &item.PolicyGeneration, &root, &item.Fingerprint, &conditions, &item.Reason, &item.DecidedAt); err != nil {
			return nil, 0, err
		}
		item.RootResource, _ = SanitizeJSON(root)
		if err := json.Unmarshal(conditions, &item.MatchedConditions); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *PGQueryStore) Context(ctx context.Context, tenant string, id int64) (*ContextV1, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	incident, err := scanIncident(tx.QueryRow(ctx, `SELECT `+incidentViewColumns+` FROM incidents WHERE tenant_id=$1 AND id=$2`, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIncidentNotFound
	}
	if err != nil {
		return nil, err
	}
	result := &ContextV1{Schema: ContextSchemaV1, Revision: incident.Version, Incident: incident, RootResource: incident.RootResource, AffectedResources: []ResourceContext{}, Signals: []SignalContext{}, Timeline: []TimelineView{}, Evidence: []map[string]any{}, CorrelationDecision: []CorrelationView{}, RawDataLinks: []string{}, Truncated: map[string]bool{}}

	rows, err := tx.Query(ctx, `SELECT s.id,s.source,s.type,s.reason,s.message,s.resource_api_version,s.resource_kind,s.namespace,s.resource_name,s.resource_uid,s.occurred_at FROM incident_signals x JOIN signals s ON s.id=x.signal_id WHERE x.incident_id=$1 ORDER BY s.occurred_at,s.id LIMIT 101`, id)
	if err != nil {
		return nil, err
	}
	resources := map[string]bool{}
	for rows.Next() {
		var item SignalContext
		if err := rows.Scan(&item.ID, &item.Source, &item.Type, &item.Reason, &item.Message, &item.Resource.APIVersion, &item.Resource.Kind, &item.Resource.Namespace, &item.Resource.Name, &item.Resource.UID, &item.OccurredAt); err != nil {
			rows.Close()
			return nil, err
		}
		if len(result.Signals) == 100 {
			result.Truncated["signals"] = true
			continue
		}
		item.Message = sanitizeText(item.Message)
		item.RawDataLink = "/api/v1/signals/" + item.ID + "/raw"
		result.Signals = append(result.Signals, item)
		result.RawDataLinks = append(result.RawDataLinks, item.RawDataLink)
		if !resources[item.Resource.UID] {
			resources[item.Resource.UID] = true
			result.AffectedResources = append(result.AffectedResources, item.Resource)
		}
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id,entry_type,actor,message,evidence,occurred_at FROM timeline_entries WHERE incident_id=$1 ORDER BY occurred_at,id LIMIT 101`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item TimelineView
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Type, &item.Actor, &item.Message, &raw, &item.OccurredAt); err != nil {
			rows.Close()
			return nil, err
		}
		if len(result.Timeline) == 100 {
			result.Truncated["timeline"] = true
			continue
		}
		item.Evidence, item.Truncated = SanitizeJSON(raw)
		result.Timeline = append(result.Timeline, item)
		if len(item.Evidence) > 0 && len(result.Evidence) < 100 {
			result.Evidence = append(result.Evidence, item.Evidence)
		}
	}
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id,signal_id,rule_name,rule_version,policy_generation,root_resource,fingerprint,matched_conditions,reason,decided_at FROM correlation_decisions WHERE incident_id=$1 ORDER BY decided_at,id LIMIT 101`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item CorrelationView
		var root, conditions []byte
		if err := rows.Scan(&item.ID, &item.SignalID, &item.RuleName, &item.RuleVersion, &item.PolicyGeneration, &root, &item.Fingerprint, &conditions, &item.Reason, &item.DecidedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if len(result.CorrelationDecision) == 100 {
			result.Truncated["correlationDecisions"] = true
			continue
		}
		item.RootResource, _ = SanitizeJSON(root)
		if err := json.Unmarshal(conditions, &item.MatchedConditions); err != nil {
			rows.Close()
			return nil, err
		}
		result.CorrelationDecision = append(result.CorrelationDecision, item)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func sanitizeText(value string) string {
	if len(value) > maxEvidenceStringBytes {
		return value[:maxEvidenceStringBytes] + "...[TRUNCATED]"
	}
	return value
}

type rowScanner interface{ Scan(...any) error }

func scanIncident(row rowScanner) (IncidentView, error) {
	var item IncidentView
	var root []byte
	err := row.Scan(&item.ID, &item.Title, &item.Status, &item.Severity, &item.Namespace, &item.Policy, &item.Category, &item.Fingerprint, &root, &item.StartedAt, &item.UpdatedAt, &item.ResolvedAt, &item.AcknowledgedAt, &item.Version, &item.PolicyGeneration)
	if err == nil {
		item.RootResource, _ = SanitizeJSON(root)
	}
	return item, err
}

func itoa(value int) string { return strconv.Itoa(value) }
