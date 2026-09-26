package incident

import "time"

const ContextSchemaV1 = "incident-context/v1"

type ResourceContext struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
}

type SignalContext struct {
	ID          string          `json:"id"`
	Source      string          `json:"source"`
	Type        string          `json:"type"`
	Reason      string          `json:"reason"`
	Message     string          `json:"message"`
	Resource    ResourceContext `json:"resource"`
	OccurredAt  time.Time       `json:"occurredAt"`
	RawDataLink string          `json:"rawDataLink"`
}

type ContextV1 struct {
	Schema              string            `json:"schema"`
	Revision            int64             `json:"revision"`
	Incident            IncidentView      `json:"incident"`
	RootResource        map[string]any    `json:"rootResource"`
	AffectedResources   []ResourceContext `json:"affectedResources"`
	Signals             []SignalContext   `json:"signals"`
	Timeline            []TimelineView    `json:"timeline"`
	Evidence            []map[string]any  `json:"evidence"`
	CorrelationDecision []CorrelationView `json:"correlationDecisions"`
	RawDataLinks        []string          `json:"rawDataLinks"`
	Truncated           map[string]bool   `json:"truncated"`
}
