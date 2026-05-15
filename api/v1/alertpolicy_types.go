package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:generate=true
type AlertPolicySeverity string
type AlertRuntimeState string

const (
	SeverityInfo     AlertPolicySeverity = "info"
	SeverityWarning  AlertPolicySeverity = "warning"
	SeverityCritical AlertPolicySeverity = "critical"

	AlertStatePending  AlertRuntimeState = "pending"
	AlertStateFiring   AlertRuntimeState = "firing"
	AlertStateResolved AlertRuntimeState = "resolved"
	AlertStateInactive AlertRuntimeState = "inactive"
)

// +kubebuilder:object:generate=true
type AlertPolicySpec struct {
	TenantID      string              `json:"tenantID"`
	PolicyName    string              `json:"policyName"`
	Query         string              `json:"query"`
	
	// +kubebuilder:validation:Type=number
	Threshold     float64             `json:"threshold"`
	Operator      string              `json:"operator"`
	Duration      string              `json:"duration,omitempty"`
	Severity      AlertPolicySeverity `json:"severity,omitempty"`
	Enabled       *bool                `json:"enabled,omitempty"`
	Labels        map[string]string   `json:"labels,omitempty"`
	Annotations   map[string]string   `json:"annotations,omitempty"`
	Notification  string              `json:"notification,omitempty"`
	RunbookURL     string              `json:"runbookURL,omitempty"`
	Owner         string              `json:"owner,omitempty"`
	EvaluationSec int                 `json:"evaluationSec,omitempty"`
}

// +kubebuilder:object:generate=true
type AlertPolicyStatus struct {
	// =========================================
	// Control Plane Status
	// =========================================

	Synced bool `json:"synced,omitempty"`

	DBID int64 `json:"dbID,omitempty"`

	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	LastError string `json:"lastError,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// =========================================
	// Runtime Plane Status
	// =========================================

	State AlertRuntimeState `json:"state,omitempty"`

	ActiveAlerts int `json:"activeAlerts,omitempty"`

	LastTriggeredAt *metav1.Time `json:"lastTriggeredAt,omitempty"`

	LastResolvedAt *metav1.Time `json:"lastResolvedAt,omitempty"`

	LastEvaluationTime *metav1.Time `json:"lastEvaluationTime,omitempty"`

	PrometheusRule string `json:"prometheusRule,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:object:generate=true
type AlertPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AlertPolicySpec   `json:"spec,omitempty"`
	Status            AlertPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:object:generate=true
type AlertPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertPolicy `json:"items"`
}
