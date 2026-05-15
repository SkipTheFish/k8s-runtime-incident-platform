package incident

import (
	"context"

	"observability-platform/internal/repository"
)

type PGRepo struct{}

func NewPGRepo() *PGRepo {
	return &PGRepo{}
}

func (r *PGRepo) Create(

	ctx context.Context,

	i Incident,

) (int64, error) {

	var id int64

	err := repository.DB.QueryRow(ctx, `
		INSERT INTO incidents (
			tenant_id,
			namespace,
			policy_name,
			alert_name,
			severity,
			status,
			resource_kind,
			resource_name,
			started_at,
			updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id
	`,
		i.Tenant,
		i.Namespace,
		i.PolicyName,
		i.AlertName,
		i.Severity,
		i.Status,
		i.ResourceKind,
		i.ResourceName,
		i.StartTime,
		i.UpdatedAt,
	).Scan(&id)

	return id, err
}

func (r *PGRepo) Close(
	ctx context.Context,
	id int64,
) error {

	_, err := repository.DB.Exec(ctx, `
		UPDATE incidents
		SET
			status = 'resolved',
			resolved_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, id)

	return err
}

func (r *PGRepo) GetOpenByTenant(

	ctx context.Context,

	tenant string,

) (*Incident, error) {

	row := repository.DB.QueryRow(ctx, `
		SELECT
			id,
			tenant_id,
			namespace,
			policy_name,
			alert_name,
			severity,
			status,
			resource_kind,
			resource_name,
			started_at,
			updated_at
		FROM incidents
		WHERE tenant_id = $1
		AND status = 'open'
		LIMIT 1
	`, tenant)

	var i Incident

	err := row.Scan(
		&i.ID,
		&i.Tenant,
		&i.Namespace,
		&i.PolicyName,
		&i.AlertName,
		&i.Severity,
		&i.Status,
		&i.ResourceKind,
		&i.ResourceName,
		&i.StartTime,
		&i.UpdatedAt,
	)

	if err != nil {
		return nil, nil
	}

	return &i, nil
}

func (r *PGRepo) ListByTenant(

	ctx context.Context,

	tenant string,

) ([]Incident, error) {

	rows, err := repository.DB.Query(ctx, `
		SELECT
			id,
			tenant_id,
			namespace,
			policy_name,
			alert_name,
			severity,
			status,
			resource_kind,
			resource_name,
			started_at,
			updated_at
		FROM incidents
		WHERE tenant_id = $1
		ORDER BY updated_at DESC
	`, tenant)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []Incident

	for rows.Next() {

		var i Incident

		err := rows.Scan(
			&i.ID,
			&i.Tenant,
			&i.Namespace,
			&i.PolicyName,
			&i.AlertName,
			&i.Severity,
			&i.Status,
			&i.ResourceKind,
			&i.ResourceName,
			&i.StartTime,
			&i.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		result = append(result, i)
	}

	return result, nil
}

func (r *PGRepo) GetByID(

	ctx context.Context,

	tenant string,

	id int64,

) (*Incident, error) {

	row := repository.DB.QueryRow(ctx, `
		SELECT
			id,
			tenant_id,
			namespace,
			policy_name,
			alert_name,
			severity,
			status,
			resource_kind,
			resource_name,
			started_at,
			updated_at
		FROM incidents
		WHERE tenant_id = $1
		AND id = $2
	`, tenant, id)

	var i Incident

	err := row.Scan(
		&i.ID,
		&i.Tenant,
		&i.Namespace,
		&i.PolicyName,
		&i.AlertName,
		&i.Severity,
		&i.Status,
		&i.ResourceKind,
		&i.ResourceName,
		&i.StartTime,
		&i.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &i, nil
}

func (r *PGRepo) ListOpenByTenant(

	ctx context.Context,

	tenant string,

) ([]Incident, error) {

	rows, err := repository.DB.Query(ctx, `
		SELECT
			id,
			tenant_id,
			namespace,
			policy_name,
			alert_name,
			severity,
			status,
			resource_kind,
			resource_name,
			started_at,
			updated_at
		FROM incidents
		WHERE tenant_id = $1
		AND status = 'open'
		ORDER BY updated_at DESC
	`, tenant)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []Incident

	for rows.Next() {

		var i Incident

		err := rows.Scan(
			&i.ID,
			&i.Tenant,
			&i.Namespace,
			&i.PolicyName,
			&i.AlertName,
			&i.Severity,
			&i.Status,
			&i.ResourceKind,
			&i.ResourceName,
			&i.StartTime,
			&i.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		result = append(result, i)
	}

	return result, nil
}