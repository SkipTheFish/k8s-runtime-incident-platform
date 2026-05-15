package incident

import "context"

type Repository interface {
	Create(ctx context.Context, i Incident) (int64, error)
	Close(ctx context.Context, id int64) error
	GetOpenByTenant(ctx context.Context, tenant string) (*Incident, error)
	ListByTenant(
		ctx context.Context,
		tenant string,
	) ([]Incident, error)

	GetByID(
		ctx context.Context,
		tenant string,
		id int64,
	) (*Incident, error)

	ListOpenByTenant(
		ctx context.Context,
		tenant string,
	) ([]Incident, error)
}

