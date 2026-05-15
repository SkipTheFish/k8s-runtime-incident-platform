package incidentfsm

import (
	"context"
	"log"
	"time"

	runtimeevent "observability-platform/internal/runtime/event"
	runtimeline "observability-platform/internal/runtime/timeline"
)

const (
	FlappingWindow = 2 * time.Minute
)

type Service struct {
	repo *Repository

	timeline *runtimeline.Repository
}

func NewService(
	repo *Repository,
	timeline *runtimeline.Repository,
) *Service {

	return &Service{
		repo: repo,

		timeline: timeline,
	}
}

func (s *Service) HandleAlertTriggered(
	ctx context.Context,
	evt runtimeevent.RuntimeEvent,
) {

	input := OpenInput{
		TenantID: evt.TenantID,

		Namespace: evt.Namespace,

		PolicyName: evt.PolicyName,

		AlertName: evt.AlertName,

		Severity: evt.Severity,

		ResourceKind: evt.Resource.Kind,

		ResourceName: evt.Resource.Name,
	}

	// =========================================
	// Existing Active Incident
	// =========================================

	active, err := s.repo.FindActiveIncident(
		ctx,
		input,
	)
	if err != nil {

		log.Printf(
			"INCIDENT FSM: find active failed: %v",
			err,
		)

		return
	}

	if active != nil {

		log.Printf(
			"INCIDENT FSM: existing active incident=%d",
			active.ID,
		)

		_ = s.repo.Touch(
			ctx,
			active.ID,
		)

		if err := s.timeline.Append(
			ctx,
			active.ID,
			evt,
		); err != nil {

			log.Printf(
				"TIMELINE: append failed incident=%d err=%v",
				active.ID,
				err,
			)
		}

		return
	}

	// =========================================
	// Recently Resolved -> Reopen
	// =========================================

	recent, err := s.repo.FindRecentlyResolved(
		ctx,
		input,
		FlappingWindow,
	)
	if err != nil {

		log.Printf(
			"INCIDENT FSM: find recently resolved failed: %v",
			err,
		)

		return
	}

	if recent != nil {

		log.Printf(
			"INCIDENT FSM: reopen incident=%d",
			recent.ID,
		)

		if err := s.repo.Reopen(
			ctx,
			recent.ID,
		); err != nil {

			log.Printf(
				"INCIDENT FSM: reopen failed: %v",
				err,
			)

			return
		}

		if err := s.timeline.Append(
			ctx,
			recent.ID,
			evt,
		); err != nil {

			log.Printf(
				"TIMELINE: append failed incident=%d err=%v",
				recent.ID,
				err,
			)
		}

		return
	}

	// =========================================
	// Create New Incident
	// =========================================

	id, err := s.repo.Create(
		ctx,
		input,
	)
	if err != nil {

		log.Printf(
			"INCIDENT FSM: create failed: %v",
			err,
		)

		return
	}

	log.Printf(
		"INCIDENT FSM: created incident=%d",
		id,
	)

	if err := s.timeline.Append(
		ctx,
		id,
		evt,
	); err != nil {

		log.Printf(
			"TIMELINE: append failed incident=%d err=%v",
			id,
			err,
		)
	}
}

func (s *Service) HandleAlertResolved(
	ctx context.Context,
	evt runtimeevent.RuntimeEvent,
) {

	input := OpenInput{
		TenantID: evt.TenantID,

		Namespace: evt.Namespace,

		PolicyName: evt.PolicyName,

		ResourceKind: evt.Resource.Kind,

		ResourceName: evt.Resource.Name,
	}

	active, err := s.repo.FindActiveIncident(
		ctx,
		input,
	)
	if err != nil {

		log.Printf(
			"INCIDENT FSM: find active failed: %v",
			err,
		)

		return
	}

	if active == nil {

		log.Printf(
			"INCIDENT FSM: no active incident",
		)

		return
	}

	if err := s.repo.Resolve(
		ctx,
		active.ID,
	); err != nil {

		log.Printf(
			"INCIDENT FSM: resolve failed: %v",
			err,
		)

		return
	}

	log.Printf(
		"INCIDENT FSM: resolved incident=%d",
		active.ID,
	)

	if err := s.timeline.Append(
		ctx,
		active.ID,
		evt,
	); err != nil {

		log.Printf(
			"TIMELINE: append failed incident=%d err=%v",
			active.ID,
			err,
		)
	}
}

func (s *Service) AttachRuntimeEvent(
	ctx context.Context,
	evt runtimeevent.RuntimeEvent,
) {

	input := OpenInput{
		TenantID: evt.TenantID,

		Namespace: evt.Namespace,

		PolicyName: evt.PolicyName,

		ResourceKind: evt.Resource.Kind,

		ResourceName: evt.Resource.Name,
	}

	active, err := s.repo.FindActiveIncident(
		ctx,
		input,
	)
	if err != nil {

		log.Printf(
			"INCIDENT FSM: attach find active failed: %v",
			err,
		)

		return
	}

	if active == nil {
		return
	}

	_ = s.repo.Touch(
		ctx,
		active.ID,
	)

	if err := s.timeline.Append(
		ctx,
		active.ID,
		evt,
	); err != nil {

		log.Printf(
			"TIMELINE: append failed incident=%d err=%v",
			active.ID,
			err,
		)
	}
}