package alert

import (
	"context"

	"observability-platform/internal/repository"
)

type PGRepo struct{}

func NewPGRepo() *PGRepo {
	return &PGRepo{}
}

func (r *PGRepo) CreatePolicy(ctx context.Context, p AlertPolicy) error {

	_, err := repository.DB.Exec(ctx, `
		INSERT INTO alert_policies 
		(tenant_id, name, query, threshold, operator, duration)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		p.Tenant,
		p.Name,
		p.Query,
		p.Threshold,
		p.Operator,
		p.Duration,
	)

	return err
}

func (r *PGRepo) ListPolicies(ctx context.Context) ([]AlertPolicy, error) {

	rows, err := repository.DB.Query(ctx, `
		SELECT id, tenant_id, name, query, threshold, operator, duration
		FROM alert_policies
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AlertPolicy

	for rows.Next() {
		var p AlertPolicy

		err := rows.Scan(
			&p.ID,
			&p.Tenant,
			&p.Name,
			&p.Query,
			&p.Threshold,
			&p.Operator,
			&p.Duration,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, p)
	}

	return result, nil
}

func (r *PGRepo) CreateInstance(ctx context.Context, policyID int64, value float64) error {

	_, err := repository.DB.Exec(ctx, `
		INSERT INTO alert_instances (policy_id, status, value, start_time)
		VALUES ($1, 'firing', $2, NOW())
	`, policyID, value)

	return err
}