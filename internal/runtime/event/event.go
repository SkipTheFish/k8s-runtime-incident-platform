package event

import "time"

type Type string

const (
	// =========================================
	// Alert Events
	// =========================================

	TypeAlertTriggered Type = "alert.triggered"
	TypeAlertResolved  Type = "alert.resolved"

	// =========================================
	// Kubernetes Events
	// =========================================

	TypeK8sWarning Type = "k8s.warning"
	TypeK8sNormal  Type = "k8s.normal"

	// =========================================
	// System Events
	// =========================================z

	TypeSystem Type = "system"
)

type RuntimeEvent struct {
	// =========================================
	// Identity
	// =========================================

	ID string

	Type Type

	Source string

	// =========================================
	// Multi-Tenant
	// =========================================

	TenantID string

	Namespace string

	// =========================================
	// Policy / Alert
	// =========================================

	PolicyName string

	AlertName string

	Severity string

	// =========================================
	// Resource
	// =========================================

	Resource RuntimeResource `json:"resource,omitempty"`

	// =========================================
	// Human Message
	// =========================================

	Message string

	// =========================================
	// Labels / Metadata
	// =========================================

	Labels map[string]string

	Metadata map[string]string

	// =========================================
	// Correlation
	// =========================================

	CorrelationID string

	ParentEventID string

	// =========================================
	// Time
	// =========================================

	OccurredAt time.Time
}
