package v1

import (
	"context"
	"fmt"
	"text/template"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var supportedCorrelationGroupBy = map[string]struct{}{
	"rootResourceUID": {}, "container": {}, "podUID": {}, "namespace": {}, "reason": {},
}

// +kubebuilder:webhook:path=/validate-observability-io-v1-correlationpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.io,resources=correlationpolicies,verbs=create;update,versions=v1,name=vcorrelationpolicy.kb.io,admissionReviewVersions=v1
type correlationPolicyValidator struct{ Client client.Client }

func (r *CorrelationPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).WithValidator(&correlationPolicyValidator{Client: mgr.GetClient()}).Complete()
}

func (v *correlationPolicyValidator) ValidateCreate(ctx context.Context, obj *CorrelationPolicy) (admission.Warnings, error) {
	return nil, v.validate(ctx, obj)
}
func (v *correlationPolicyValidator) ValidateUpdate(ctx context.Context, _, newObject *CorrelationPolicy) (admission.Warnings, error) {
	return nil, v.validate(ctx, newObject)
}
func (v *correlationPolicyValidator) ValidateDelete(context.Context, *CorrelationPolicy) (admission.Warnings, error) {
	return nil, nil
}

func (v *correlationPolicyValidator) validate(ctx context.Context, policy *CorrelationPolicy) error {
	root := field.NewPath("spec")
	var errs field.ErrorList
	if len(policy.Spec.GroupBy) == 0 {
		errs = append(errs, field.Required(root.Child("groupBy"), "at least one grouping field is required"))
	}
	for index, value := range policy.Spec.GroupBy {
		if _, ok := supportedCorrelationGroupBy[value]; !ok {
			errs = append(errs, field.NotSupported(root.Child("groupBy").Index(index), value, sortedGroupByFields()))
		}
	}
	validateWindow := func(name, value string, min, max time.Duration) {
		duration, err := time.ParseDuration(value)
		if err != nil || duration < min || duration > max {
			errs = append(errs, field.Invalid(root.Child(name), value, fmt.Sprintf("must be from %s through %s", min, max)))
		}
	}
	validateWindow("window", policy.Spec.Window, time.Second, 24*time.Hour)
	validateWindow("reopenWindow", policy.Spec.ReopenWindow, time.Second, 7*24*time.Hour)
	if policy.Spec.FingerprintTemplate != "" {
		if _, err := template.New("fingerprint").Option("missingkey=error").Parse(policy.Spec.FingerprintTemplate); err != nil {
			errs = append(errs, field.Invalid(root.Child("fingerprintTemplate"), "<redacted>", "template cannot be compiled"))
		}
	}
	if policy.Spec.Enabled && v.Client != nil {
		var existing CorrelationPolicyList
		if err := v.Client.List(ctx, &existing, client.InNamespace(policy.Namespace)); err != nil {
			return fmt.Errorf("list correlation policies: %w", err)
		}
		for i := range existing.Items {
			other := &existing.Items[i]
			if other.Name == policy.Name || !other.Spec.Enabled || other.Spec.Priority != policy.Spec.Priority {
				continue
			}
			if policyDomainsOverlap(policy.Spec, other.Spec) {
				errs = append(errs, field.Invalid(root.Child("priority"), policy.Spec.Priority, fmt.Sprintf("matching domain conflicts with policy %q at the same priority", other.Name)))
				break
			}
		}
	}
	if len(errs) > 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "CorrelationPolicy"}, policy.Name, errs)
	}
	return nil
}

func policyDomainsOverlap(a, b CorrelationPolicySpec) bool {
	return stringSetsOverlap(a.Sources, b.Sources, false) && stringSetsOverlap(a.EventReasons, b.EventReasons, true) && stringSetsOverlap(a.Namespaces, b.Namespaces, true)
}
func stringSetsOverlap[T ~string](a, b []T, emptyWildcard bool) bool {
	if emptyWildcard && (len(a) == 0 || len(b) == 0) {
		return true
	}
	for _, left := range a {
		for _, right := range b {
			if left == right {
				return true
			}
		}
	}
	return false
}
func sortedGroupByFields() []string {
	return []string{"container", "namespace", "podUID", "reason", "rootResourceUID"}
}
