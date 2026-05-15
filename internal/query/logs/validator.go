package logs

import (
	"errors"
	"strings"

	"observability-platform/internal/query"
)

type LogQLValidator struct{}

func (v *LogQLValidator) Validate(q *query.Query) error {
	if q.Expr == "" {
		return errors.New("logql empty")
	}

	if q.Tenant == "" {
		return errors.New("tenant missing")
	}

	// 基础防注入
	if strings.Contains(q.Expr, ";") {
		return errors.New("invalid logql")
	}

	return nil
}