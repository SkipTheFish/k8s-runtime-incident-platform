package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ObservabilityTargetSpec struct {
	TenantID string `json:"tenantID"`

	ServiceName string `json:"serviceName"`

	Namespace string `json:"namespace"`

	Selector map[string]string `json:"selector"`

	PortName string `json:"portName"`

	ScrapePath string `json:"scrapePath,omitempty"`

	ScrapeInterval string `json:"scrapeInterval,omitempty"`
}

type ObservabilityTargetStatus struct {
	Synced bool `json:"synced,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type ObservabilityTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObservabilityTargetSpec   `json:"spec,omitempty"`
	Status ObservabilityTargetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type ObservabilityTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObservabilityTarget `json:"items"`
}