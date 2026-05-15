package timeline

import (
	"context"

	"observability-platform/internal/repository"
)

type PGRepo struct{}

func NewPGRepo() *PGRepo {
	return &PGRepo{}
}

func (r *PGRepo) AddEvent(ctx context.Context, e TimelineEvent) error {

	_, err := repository.DB.Exec(ctx, `
		INSERT INTO timeline_events (incident_id, type, message,resource_kind,resource_name)
		VALUES ($1, $2, $3, $4, $5)
	`,
		e.IncidentID,
		e.Type,
		e.Message,
		e.ResourceKind,
		e.ResourceName,
	)

	return err
}

func (r *PGRepo) ListByIncident(ctx context.Context, incidentID int64) ([]TimelineEvent, error) {

	rows, err := repository.DB.Query(ctx, `
		SELECT
			id,
			incident_id,
			event_type,
			message,
			created_at,resource_kind,resource_name
		FROM runtime_timeline
		WHERE incident_id = $1
		ORDER BY created_at
	`, incidentID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TimelineEvent

	for rows.Next() {
		var e TimelineEvent

		err := rows.Scan(
			&e.ID,
			&e.IncidentID,
			&e.Type,
			&e.Message,
			&e.Timestamp,
			&e.ResourceKind,
			&e.ResourceName,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, e)
	}

	return result, nil
}