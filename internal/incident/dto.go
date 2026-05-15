package incident

import "time"

type IncidentDTO struct {

	ID int64 `json:"id"`

	Title string `json:"title"`

	Status string `json:"status"`

	Severity string `json:"severity"`

	ResourceKind string `json:"resource_kind"`

	ResourceName string `json:"resource_name"`

	StartedAt time.Time `json:"started_at"`

	UpdatedAt time.Time `json:"updated_at"`
}