package incidentfsm

import (
	"context"
	"time"
	"errors"

	"observability-platform/internal/repository"
	"github.com/jackc/pgx/v5"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

type OpenInput struct {
	TenantID string

	Namespace string

	PolicyName string

	AlertName string

	Severity string

	ResourceKind string

	ResourceName string
}

type Incident struct {
	ID int64

	Status Status

	ResolvedAt *time.Time
}

func (r *Repository) FindActiveIncident(
	ctx context.Context,
	in OpenInput,
) (*Incident, error) {

	row := repository.DB.QueryRow(ctx, `
		SELECT id, status, resolved_at
		FROM incidents
		WHERE tenant_id = $1
		  AND namespace = $2
		  AND policy_name = $3
		  AND COALESCE(resource_kind, '') = COALESCE($4, '')
		  AND COALESCE(resource_name, '') = COALESCE($5, '')
		  AND status IN ('open', 'acknowledged', 'suppressed', 'reopened')
		ORDER BY started_at DESC
		LIMIT 1
	`,
		in.TenantID,
		in.Namespace,
		in.PolicyName,
		in.ResourceKind,
		in.ResourceName,
	)

	var incident Incident

	err := row.Scan(
		&incident.ID,
		&incident.Status,
		&incident.ResolvedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &incident, nil
}

func (r *Repository) FindRecentlyResolved(
	ctx context.Context,
	in OpenInput,
	window time.Duration,
) (*Incident, error) {

	row := repository.DB.QueryRow(ctx, `
		SELECT id, status, resolved_at
		FROM incidents
		WHERE tenant_id = $1
		  AND namespace = $2
		  AND policy_name = $3
		  AND COALESCE(resource_kind, '') = COALESCE($4, '')
		  AND COALESCE(resource_name, '') = COALESCE($5, '')
		  AND status = 'resolved'
		  AND resolved_at >= NOW() - ($6 * interval '1 second')
		ORDER BY resolved_at DESC
		LIMIT 1
	`,
		in.TenantID,
		in.Namespace,
		in.PolicyName,
		in.ResourceKind,
		in.ResourceName,
		int(window.Seconds()),
	)

	var incident Incident

	err := row.Scan(
		&incident.ID,
		&incident.Status,
		&incident.ResolvedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &incident, nil
}

func (r *Repository) Create(
	ctx context.Context,
	in OpenInput,
) (int64, error) {

	var id int64

	err := repository.DB.QueryRow(ctx, `
		INSERT INTO incidents
		(
			tenant_id,
			namespace,
			policy_name,
			alert_name,
			severity,
			resource_kind,
			resource_name,
			status,
			started_at,
			last_event_at,
			updated_at
		)
		VALUES
		(
			$1,$2,$3,$4,$5,$6,$7,
			'open',
			NOW(),
			NOW(),
			NOW()
		)
		RETURNING id
	`,
		in.TenantID,
		in.Namespace,
		in.PolicyName,
		in.AlertName,
		in.Severity,
		in.ResourceKind,
		in.ResourceName,
	).Scan(&id)

	return id, err
}

func (r *Repository) Reopen(
	ctx context.Context,
	id int64,
) error {

	_, err := repository.DB.Exec(ctx, `
		UPDATE incidents
		SET status = 'reopened',
		    reopen_count = reopen_count + 1,
		    resolved_at = NULL,
		    updated_at = NOW(),
		    last_event_at = NOW()
		WHERE id = $1
	`, id)

	return err
}

func (r *Repository) Resolve(
	ctx context.Context,
	id int64,
) error {

	_, err := repository.DB.Exec(ctx, `
		UPDATE incidents
		SET status = 'resolved',
		    resolved_at = NOW(),
		    updated_at = NOW(),
		    last_event_at = NOW()
		WHERE id = $1
	`, id)

	return err
}

func (r *Repository) Touch(
	ctx context.Context,
	id int64,
) error {

	_, err := repository.DB.Exec(ctx, `
		UPDATE incidents
		SET last_event_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, id)

	return err
}

func (r *Repository) Acknowledge(
	ctx context.Context,
	id int64,
) error {

	_, err := repository.DB.Exec(ctx, `
		UPDATE incidents
		SET status = 'acknowledged',
		    acknowledged_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'open'
	`, id)

	return err
}