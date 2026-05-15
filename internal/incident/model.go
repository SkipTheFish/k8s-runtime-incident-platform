package incident

import "time"

type IncidentStatus string

const (
	IncidentOpen     IncidentStatus = "open"
	IncidentResolved IncidentStatus = "resolved"
)

type Incident struct {

	ID int64

	Tenant string

	Namespace string

	PolicyName string

	AlertName string

	Title string

	Severity string

	Status IncidentStatus

	ResourceKind string

	ResourceName string

	StartTime time.Time

	ResolvedAt *time.Time

	UpdatedAt time.Time
}