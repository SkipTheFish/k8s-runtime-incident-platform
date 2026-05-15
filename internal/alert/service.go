package alert

import "context"

type Service struct {
	repo Repository
}

func NewService(r Repository) *Service {
	return &Service{repo: r}
}

func (s *Service) CreatePolicy(ctx context.Context, p AlertPolicy) error {
	return s.repo.CreatePolicy(ctx, p)
}