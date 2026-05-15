package correlation

import (
	"context"
	"log"

	runtimeevent "observability-platform/internal/runtime/event"
	runtimegraph "observability-platform/internal/runtime/graph"
	"observability-platform/internal/runtime/incidentfsm"
)

type Engine struct {
	bus       *runtimeevent.Bus
	fsm       *incidentfsm.Service
	graphRepo *runtimegraph.Repository
}

func NewEngine(
	bus *runtimeevent.Bus,
	fsm *incidentfsm.Service,
	graphRepo *runtimegraph.Repository,
) *Engine {
	return &Engine{
		bus:       bus,
		fsm:       fsm,
		graphRepo: graphRepo,
	}
}

func (e *Engine) Start(ctx context.Context) {

	log.Println("CORRELATION: engine started")

	ch := e.bus.Subscribe(1024)

	for {
		select {

		case evt := <-ch:
			e.handle(ctx, evt)

		case <-ctx.Done():

			log.Println("CORRELATION: engine stopped")

			return
		}
	}
}

func (e *Engine) handle(ctx context.Context, evt runtimeevent.RuntimeEvent) {
	log.Printf(
		"CORRELATION: event=%s namespace=%s policy=%s resource=%s/%s",
		evt.Type,
		evt.Namespace,
		evt.PolicyName,
		evt.Resource.Kind,
		evt.Resource.Name,
	)

	e.updateGraph(ctx, evt)

	switch evt.Type {
	case runtimeevent.TypeAlertTriggered:
		e.fsm.HandleAlertTriggered(ctx, evt)

	case runtimeevent.TypeAlertResolved:
		e.fsm.HandleAlertResolved(ctx, evt)

	case runtimeevent.TypeK8sWarning:
		e.fsm.AttachRuntimeEvent(ctx, evt)

	case runtimeevent.TypeK8sNormal:
		// ignore by default
	}
}

func (e *Engine) updateGraph(ctx context.Context, evt runtimeevent.RuntimeEvent) {
	if evt.PolicyName != "" {
		_ = e.graphRepo.UpsertNode(
			ctx,
			"AlertPolicy",
			evt.Namespace,
			evt.PolicyName,
			evt.TenantID,
		)
	}

	if evt.Resource.Kind != "" && evt.Resource.Name != "" {
		_ = e.graphRepo.UpsertNode(
			ctx,
			evt.Resource.Kind,
			evt.Namespace,
			evt.Resource.Name,
			evt.TenantID,
		)
	}

	if evt.PolicyName != "" && evt.Resource.Kind != "" && evt.Resource.Name != "" {
		_ = e.graphRepo.UpsertEdge(
			ctx,
			"AlertPolicy",
			evt.Namespace,
			evt.PolicyName,
			evt.Resource.Kind,
			evt.Namespace,
			evt.Resource.Name,
			"correlates_with",
		)
	}
}

func (e *Engine) correlateK8sWarning(ctx context.Context, evt runtimeevent.RuntimeEvent) {
	// v1: 先按 namespace 找 open incident
	// v2: 如果 graph 已有关系，再走 graph
	e.fsm.AttachRuntimeEvent(ctx, evt)
}