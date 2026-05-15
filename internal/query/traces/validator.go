package traces

import (
	"errors"

	"observability-platform/internal/query"
)

type TraceValidator struct{}

func (v *TraceValidator) Validate(q *query.Query) error {

	if q.Tenant == "" {
		return errors.New("tenant missing")
	}

	if q.Expr == "" {
		return errors.New("trace expr empty")
	}

	return nil
}