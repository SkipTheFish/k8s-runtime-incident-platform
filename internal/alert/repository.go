package alert

import (
	"context"
)

type Repository interface {
	ListPolicies(ctx context.Context) ([]AlertPolicy, error)

	CreatePolicy(ctx context.Context, p AlertPolicy) error   // ✅ 补这个

	CreateInstance(ctx context.Context, policyID int64, value float64) error
}