package alert

import (
	"context"
	"log"
	"time"

	"observability-platform/internal/query"
	"observability-platform/internal/incident"
)

type Engine struct {
	queryService query.QueryService
	repo         Repository
	incidentService *incident.Service
}

func NewEngine(q query.QueryService, r Repository, i *incident.Service) *Engine {
	return &Engine{
		queryService:    q,
		repo:            r,
		incidentService: i,
	}
}

func (e *Engine) Start(ctx context.Context) {

	log.Println("ALERT ENGINE: started")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {

		case <-ticker.C:
			e.evaluate(ctx)

		case <-ctx.Done():

			log.Println("ALERT ENGINE: stopped")

			return
		}
	}
}

func (e *Engine) evaluate(ctx context.Context) {
	// log.Println("🔥 engine evaluate running")
	policies, err := e.repo.ListPolicies(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	for _, p := range policies {

		// log.Println("policies:", len(policies))

		q := &query.Query{
			Type:   query.QueryTypeMetrics,
			Mode:   query.QueryModeInstant,
			Expr:   p.Query,
			Tenant: p.Tenant,
		}

		res, err := e.queryService.Query(ctx, q)
		if err != nil {
			continue
		}

		value := extractValue(res)


		if Compare(value, p.Operator, p.Threshold) {
			e.repo.CreateInstance(ctx, p.ID, value)

			if e.incidentService != nil {
				e.incidentService.HandleAlert(ctx, p.Tenant, p.Name)
			}
		}
	}
}

func extractValue(res *query.QueryResult) float64 {
	// 简化：假设 Prometheus 返回 vector
	return 1.0
}