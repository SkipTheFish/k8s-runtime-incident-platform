// +kubebuilder:object:generate=true
// +groupName=observability.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "observability.io", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	// 【关键】必须在这里把你的所有 CRD 类型注册到 Builder 里
	SchemeBuilder.Register(
		&AlertPolicy{}, &AlertPolicyList{},
		&CorrelationPolicy{}, &CorrelationPolicyList{},
		&ObservabilityTarget{}, &ObservabilityTargetList{},
		&ObserveTenant{}, &ObserveTenantList{},
	)
}
