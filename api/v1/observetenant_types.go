package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ObserveTenantSpec struct {
	TenantID string `json:"tenantID"`

	DisplayName string `json:"displayName"`

	Owner string `json:"owner"`

	Environment string `json:"environment,omitempty"`

	RetentionDays int `json:"retentionDays,omitempty"`

	Enabled bool `json:"enabled,omitempty"`
}

type ObserveTenantStatus struct {
	NamespaceCreated bool `json:"namespaceCreated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type ObserveTenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObserveTenantSpec   `json:"spec,omitempty"`
	Status ObserveTenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type ObserveTenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObserveTenant `json:"items"`
}