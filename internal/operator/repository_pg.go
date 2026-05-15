package operator

import (
	"context"
	"fmt"

	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/repository"
)

type PGRepository struct{}

func NewPGRepository() *PGRepository {
	return &PGRepository{}
}

func (r *PGRepository) UpsertAlertPolicy(ctx context.Context, cr *obsv1.AlertPolicy) (int64, error) {
	var id int64

    // 调试：打印一下到底传了什么进去，防止传空值
    fmt.Printf("DEBUG: Upserting Name=%s, Namespace=%s\n", cr.Spec.PolicyName, cr.Namespace)

	err := repository.DB.QueryRow(ctx, `
		INSERT INTO alert_policies
		(tenant_id, name, query, threshold, operator, duration, k8s_namespace, k8s_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (k8s_namespace, k8s_name) -- 这里必须在 DB 中有对应的 UNIQUE 约束
		DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			name = EXCLUDED.name,
			query = EXCLUDED.query,
			threshold = EXCLUDED.threshold,
			operator = EXCLUDED.operator,
			duration = EXCLUDED.duration
		RETURNING id
	`,
		cr.Spec.TenantID,
		cr.Spec.PolicyName, // 对应数据库的 name 列
		cr.Spec.Query,
		cr.Spec.Threshold,
		cr.Spec.Operator,
		cr.Spec.Duration,   // 别传 0，传 Spec 里的值
		cr.Namespace,
		cr.Name,
	).Scan(&id)

	return id, err
}

func (r *PGRepository) DeleteAlertPolicy(ctx context.Context, namespace, name string) error {
	_, err := repository.DB.Exec(ctx, `
		DELETE FROM alert_policies
		WHERE k8s_namespace = $1 AND k8s_name = $2
	`, namespace, name)

	return err
}

func (r *PGRepository) GetAlertPolicyID(ctx context.Context, namespace, name string) (int64, bool, error) {
	var id int64

	err := repository.DB.QueryRow(ctx, `
		SELECT id FROM alert_policies
		WHERE k8s_namespace = $1 AND k8s_name = $2
	`, namespace, name).Scan(&id)

	if err != nil {
		return 0, false, nil
	}

	return id, true, nil
}