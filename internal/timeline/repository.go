package timeline

import "context"

type Repository interface {
	AddEvent(ctx context.Context, e TimelineEvent) error
	ListByIncident(ctx context.Context, incidentID int64) ([]TimelineEvent, error)
}