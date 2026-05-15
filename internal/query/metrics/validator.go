package metrics

import (
	"errors"
	"strings"

	"observability-platform/internal/query"
)

type PromQLValidator struct{}

func (v *PromQLValidator) Validate(q *query.Query) error {
	if q == nil {
		return errors.New("query is nil")
	}

	if q.Tenant == "" {
		return errors.New("tenant is empty")
	}

	if q.Expr == "" {
		return errors.New("promql expr is empty")
	}

	dangerous := []string{
		";",
		"DROP ",
		"DELETE ",
		"INSERT ",
		"UPDATE ",
	}

	upper := strings.ToUpper(q.Expr)

	for _, item := range dangerous {
		if strings.Contains(upper, item) {
			return errors.New("invalid promql expression")
		}
	}

	return nil
}