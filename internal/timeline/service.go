package timeline

import "context"

type Service interface {

	GetByIncidentID(
		ctx context.Context,
		incidentID int64,
	) ([]TimelineEventDTO, error)
}

type service struct {
	repo Repository
}

func NewService(
	r Repository,
) Service {

	return &service{
		repo: r,
	}
}

func (s *service) GetByIncidentID(

	ctx context.Context,

	incidentID int64,

) ([]TimelineEventDTO, error) {

	events, err := s.repo.ListByIncident(
		ctx,
		incidentID,
	)

	if err != nil {
		return nil, err
	}

	return ToDTO(events), nil
}