package operator

import (
	"context"

	"observability-platform/internal/alert"
)

type Controller struct {
	repo alert.Repository
}

func NewController(r alert.Repository) *Controller {
	return &Controller{repo: r}
}

func (c *Controller) Sync(name, query string, threshold float64, operator string) {

	// 👉 写入 DB（和 API 一样）
	c.repo.CreatePolicy(context.Background(), alert.AlertPolicy{
		Name:      name,
		Query:     query,
		Threshold: threshold,
		Operator:  operator,
		Tenant:    "k8s", // 默认租户
	})
}