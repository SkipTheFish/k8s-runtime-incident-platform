package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:validation:Enum=Kubernetes;Prometheus
type CorrelationSource string
type CorrelationSeverity string

const (
	CorrelationSourceKubernetes CorrelationSource   = "Kubernetes"
	CorrelationSourcePrometheus CorrelationSource   = "Prometheus"
	CorrelationSeverityInfo     CorrelationSeverity = "info"
	CorrelationSeverityWarning  CorrelationSeverity = "warning"
	CorrelationSeverityCritical CorrelationSeverity = "critical"
)

// CorrelationPolicySpec defines deterministic signal grouping. It deliberately
// has no query or threshold fields from AlertPolicy.
type CorrelationPolicySpec struct {
	Enabled bool `json:"enabled"`
	// +kubebuilder:validation:MinItems=1
	Sources      []CorrelationSource `json:"sources"`
	EventReasons []string            `json:"eventReasons,omitempty"`
	Namespaces   []string            `json:"namespaces,omitempty"`
	// +kubebuilder:validation:MinItems=1
	GroupBy []string `json:"groupBy"`
	// +kubebuilder:validation:Pattern=`^[1-9][0-9]*(ms|s|m|h)$`
	Window string `json:"window"`
	// +kubebuilder:validation:Pattern=`^[1-9][0-9]*(ms|s|m|h)$`
	ReopenWindow string `json:"reopenWindow"`
	// +kubebuilder:validation:Enum=info;warning;critical
	Severity CorrelationSeverity `json:"severity"`
	// +kubebuilder:validation:Minimum=-1000
	// +kubebuilder:validation:Maximum=1000
	Priority            int32  `json:"priority"`
	Explain             bool   `json:"explain,omitempty"`
	FingerprintTemplate string `json:"fingerprintTemplate,omitempty"`
}

type CorrelationPolicyStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	CompiledVersion    string             `json:"compiledVersion,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cpol
type CorrelationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CorrelationPolicySpec   `json:"spec,omitempty"`
	Status            CorrelationPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type CorrelationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CorrelationPolicy `json:"items"`
}
