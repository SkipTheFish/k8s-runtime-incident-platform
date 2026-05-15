package incidentfsm

type Status string

const (
	StatusOpen         Status = "open"

	StatusAcknowledged Status = "acknowledged"

	StatusSuppressed   Status = "suppressed"

	StatusResolved     Status = "resolved"

	StatusReopened     Status = "reopened"
)