package operator

import (
	"context"

	obsv1 "observability-platform/api/v1"
)

type Repository interface {
	UpsertAlertPolicy(ctx context.Context, cr *obsv1.AlertPolicy) (int64, error)
	DeleteAlertPolicy(ctx context.Context, namespace, name string) error
	GetAlertPolicyID(ctx context.Context, namespace, name string) (int64, bool, error)
}