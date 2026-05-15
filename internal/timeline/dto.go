package timeline

import "time"

type TimelineEventDTO struct {
	ID           int64     `json:"id"`

	IncidentID   int64     `json:"incident_id"`

	EventType    string    `json:"event_type"`

	TimelineType string    `json:"timeline_type"`

	Severity     string    `json:"severity"`

	Message      string    `json:"message"`

	ResourceKind string    `json:"resource_kind"`

	ResourceName string    `json:"resource_name"`

	OccurredAt   time.Time `json:"occurred_at"`
}