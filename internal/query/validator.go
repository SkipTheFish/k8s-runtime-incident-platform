package query

import "errors"

type QueryValidator interface {
	Validate(q *Query) error
}

type DefaultValidator struct{}

func (v *DefaultValidator) Validate(q *Query) error {
	if q == nil {
		return errors.New("query is nil")
	}
	if q.Expr == "" {
		return errors.New("query expr is empty")
	}
	if q.Tenant == "" {
		return errors.New("tenant is empty")
	}
	if q.Type == "" {
		return errors.New("query type is empty")
	}
	return nil
}