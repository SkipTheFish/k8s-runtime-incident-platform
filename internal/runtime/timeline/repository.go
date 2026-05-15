package timeline

import (
	"context"
	"crypto/sha1"
	"fmt"
	"time"

	"observability-platform/internal/repository"

	runtimeevent "observability-platform/internal/runtime/event"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Append(
	ctx context.Context,
	incidentID int64,
	evt runtimeevent.RuntimeEvent,
) error {

	groupKey := buildGroupKey(evt)

	// =========================================
	// Try Group Existing
	// =========================================

	result, err := repository.DB.Exec(ctx, `
		UPDATE runtime_timeline
		SET
		    event_count = event_count + 1,
		    last_occurred_at = NOW()
		WHERE incident_id = $1
		  AND group_key = $2
		  AND created_at >= NOW() - interval '2 minutes'
	`,
		incidentID,
		groupKey,
	)

	if err == nil && result.RowsAffected() > 0 {
		return nil
	}

	// =========================================
	// Insert New Timeline Entry
	// =========================================

	_, err = repository.DB.Exec(ctx, `
		INSERT INTO runtime_timeline
		(
			incident_id,
			tenant_id,
			namespace,
			source,
			event_type,
			resource_kind,
			resource_name,
			policy_name,
			message,

			timeline_type,
			group_key,
			event_count,
			first_occurred_at,
			last_occurred_at,

			created_at
		)
		VALUES
		(
			$1,$2,$3,$4,$5,$6,$7,$8,$9,
			$10,$11,1,NOW(),NOW(),
			NOW()
		)
	`,
		incidentID,
		evt.TenantID,
		evt.Namespace,
		evt.Source,
		string(evt.Type),
		evt.Resource.Kind,
		evt.Resource.Name,
		evt.PolicyName,
		evt.Message,

		classify(evt),
		groupKey,
	)

	return err
}

func classify(evt runtimeevent.RuntimeEvent) EntryType {

	switch evt.Type {

	case runtimeevent.TypeAlertTriggered:
		return EntryTypeSymptom

	case runtimeevent.TypeAlertResolved:
		return EntryTypeSystem

	case runtimeevent.TypeK8sWarning:
		return EntryTypeCause

	case runtimeevent.TypeK8sNormal:
		return EntryTypeContext

	default:
		return EntryTypeSystem
	}
}

func buildGroupKey(evt runtimeevent.RuntimeEvent) string {

	raw := fmt.Sprintf(
		"%s|%s|%s|%s|%s",
		evt.Type,
		evt.Namespace,
		evt.Resource.Kind,
		evt.Resource.Name,
		evt.Message,
	)

	return fmt.Sprintf(
		"%x",
		sha1.Sum([]byte(raw)),
	)
}

type Entry struct {
	ID int64 `json:"id"`

	IncidentID int64 `json:"incidentID"`

	TimelineType EntryType `json:"timelineType"`

	EventType string `json:"eventType"`

	ResourceKind string `json:"resourceKind"`

	ResourceName string `json:"resourceName"`

	Message string `json:"message"`

	EventCount int `json:"eventCount"`

	CreatedAt time.Time `json:"createdAt"`
}

func (r *Repository) ListByIncident(
	ctx context.Context,
	incidentID int64,
	limit int,
) ([]Entry, error) {

	rows, err := repository.DB.Query(ctx, `
		SELECT
		    id,
		    incident_id,
		    timeline_type,
		    event_type,
		    resource_kind,
		    resource_name,
		    message,
		    event_count,
		    created_at
		FROM runtime_timeline
		WHERE incident_id = $1
		ORDER BY created_at ASC
		LIMIT $2
	`,
		incidentID,
		limit,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := make([]Entry, 0)

	for rows.Next() {

		var entry Entry

		if err := rows.Scan(
			&entry.ID,
			&entry.IncidentID,
			&entry.TimelineType,
			&entry.EventType,
			&entry.ResourceKind,
			&entry.ResourceName,
			&entry.Message,
			&entry.EventCount,
			&entry.CreatedAt,
		); err != nil {
			return nil, err
		}

		result = append(result, entry)
	}

	return result, nil
}