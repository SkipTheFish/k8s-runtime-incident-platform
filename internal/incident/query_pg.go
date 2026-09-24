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

type rowScanner interface{ Scan(...any) error }

func scanIncident(row rowScanner) (IncidentView, error) {
	var item IncidentView
	var root []byte
	err := row.Scan(&item.ID, &item.Title, &item.Status, &item.Severity, &item.Namespace, &item.Policy, &item.Category, &item.Fingerprint, &root, &item.StartedAt, &item.UpdatedAt, &item.ResolvedAt, &item.AcknowledgedAt, &item.Version, &item.PolicyGeneration)
	if err == nil {
		err = json.Unmarshal(root, &item.RootResource)
	}
	return item, err
}

func itoa(value int) string { return strconv.Itoa(value) }
