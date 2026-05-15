package incident

import (
	"context"
	"time"

	"observability-platform/internal/timeline"
)

type Service struct {

	repo Repository

	timeline timeline.Repository
}

func NewService(

	r Repository,

	t timeline.Repository,

) *Service {

	return &Service{
		repo: r,
		timeline: t,
	}
}

func (s *Service) List(

	ctx context.Context,

	tenant string,

) ([]IncidentDTO, error) {

	incidents, err := s.repo.ListByTenant(
		ctx,
		tenant,
	)

	if err != nil {
		return nil, err
	}

	result := make(
		[]IncidentDTO,
		0,
		len(incidents),
	)

	for _, inc := range incidents {
		result = append(
			result,
			ToDTO(inc),
		)
	}

	return result, nil
}

func (s *Service) Get(

	ctx context.Context,

	tenant string,

	id int64,

) (*IncidentDTO, error) {

	inc, err := s.repo.GetByID(
		ctx,
		tenant,
		id,
	)

	if err != nil {
		return nil, err
	}

	dto := ToDTO(*inc)

	return &dto, nil
}

func (s *Service) ListOpen(

	ctx context.Context,

	tenant string,

) ([]IncidentDTO, error) {

	incidents, err := s.repo.ListOpenByTenant(
		ctx,
		tenant,
	)

	if err != nil {
		return nil, err
	}

	result := make(
		[]IncidentDTO,
		0,
		len(incidents),
	)

	for _, inc := range incidents {

		result = append(
			result,
			ToDTO(inc),
		)
	}

	return result, nil
}

func (s *Service) HandleAlert(

	ctx context.Context,

	tenant string,

	message string,

) error {

	inc, _ := s.repo.GetOpenByTenant(
		ctx,
		tenant,
	)

	if inc == nil {

		id, err := s.repo.Create(
			ctx,
			Incident{
				Tenant: tenant,

				Title: message,

				Status: IncidentOpen,

				StartTime: time.Now(),
			},
		)

		if err != nil {
			return err
		}

		return s.timeline.AddEvent(
			ctx,
			timeline.TimelineEvent{
				IncidentID: id,

				Type: timeline.EventAlertFired,

				Message: message,

				Timestamp: time.Now(),
			},
		)
	}

	return s.timeline.AddEvent(
		ctx,
		timeline.TimelineEvent{
			IncidentID: inc.ID,

			Type: timeline.EventAlertFired,

			Message: message,

			Timestamp: time.Now(),
		},
	)
}