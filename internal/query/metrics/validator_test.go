package metrics

import (
	"testing"

	"observability-platform/internal/query"
)

func TestPromQLValidatorSuccess(t *testing.T) {
	v := &PromQLValidator{}

	err := v.Validate(&query.Query{
		Type:   query.QueryTypeMetrics,
		Expr:   "up",
		Tenant: "t1",
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestPromQLValidatorEmptyExpr(t *testing.T) {
	v := &PromQLValidator{}

	err := v.Validate(&query.Query{
		Type:   query.QueryTypeMetrics,
		Expr:   "",
		Tenant: "t1",
	})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPromQLValidatorEmptyTenant(t *testing.T) {
	v := &PromQLValidator{}

	err := v.Validate(&query.Query{
		Type:   query.QueryTypeMetrics,
		Expr:   "up",
		Tenant: "",
	})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPromQLValidatorDangerousExpr(t *testing.T) {
	v := &PromQLValidator{}

	err := v.Validate(&query.Query{
		Type:   query.QueryTypeMetrics,
		Expr:   "up; DROP TABLE users",
		Tenant: "t1",
	})

	if err == nil {
		t.Fatal("expected error")
	}
}