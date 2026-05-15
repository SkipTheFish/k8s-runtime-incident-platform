package v1

import (
	"fmt"
	"net/url"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultDuration      = "1m"
	DefaultSeverity      = "warning"
	DefaultEvaluationSec = 30
)

//+kubebuilder:webhook:path=/mutate-observability-io-v1-alertpolicy,mutating=true,failurePolicy=fail,sideEffects=None,groups=observability.io,resources=alertpolicies,verbs=create;update,versions=v1,name=malertpolicy.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-observability-io-v1-alertpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.io,resources=alertpolicies,verbs=create;update;delete,versions=v1,name=valertpolicy.kb.io,admissionReviewVersions=v1

func (r *AlertPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr,r).
		Complete()
}

func (r *AlertPolicy) Default() {
	if r.Spec.Duration == "" {
		r.Spec.Duration = DefaultDuration
	}
	if r.Spec.Severity == "" {
		r.Spec.Severity = DefaultSeverity
	}
	if r.Spec.Enabled == nil {
		v := true
		r.Spec.Enabled = &v
	}
	if r.Spec.EvaluationSec == 0 {
		r.Spec.EvaluationSec = DefaultEvaluationSec
	}
	if r.Spec.Labels == nil {
		r.Spec.Labels = map[string]string{}
	}
	if r.Spec.Annotations == nil {
		r.Spec.Annotations = map[string]string{}
	}
}

func (r *AlertPolicy) ValidateCreate() error {
	return r.validate()
}

func (r *AlertPolicy) ValidateUpdate(old runtime.Object) error {
	return r.validate()
}

func (r *AlertPolicy) ValidateDelete() error {
	return nil
}

func (r *AlertPolicy) validate() error {
	var errs field.ErrorList

	if r.Spec.PolicyName == "" {
		errs = append(errs, field.Required(field.NewPath("spec").Child("policyName"), "policyName is required"))
	}

	if r.Spec.Query == "" {
		errs = append(errs, field.Required(field.NewPath("spec").Child("query"), "query is required"))
	}

	if r.Spec.Operator != ">" &&
		r.Spec.Operator != "<" &&
		r.Spec.Operator != ">=" &&
		r.Spec.Operator != "<=" &&
		r.Spec.Operator != "==" &&
		r.Spec.Operator != "!=" {
		errs = append(errs, field.NotSupported(
			field.NewPath("spec").Child("operator"),
			r.Spec.Operator,
			[]string{">", "<", ">=", "<=", "==", "!="},
		))
	}

	// 3. 增强：Duration 格式校验
	// 防止用户填了数据库能收、但 Prometheus 不认的乱码
	if r.Spec.Duration != "" {
		// 简单的正则示例：数字 + [smhdwy]
		match, _ := regexp.MatchString(`^[0-9]+[smhdwy]$`, r.Spec.Duration)
		if !match {
			errs = append(errs, field.Invalid(
				field.NewPath("spec").Child("duration"),
				r.Spec.Duration,
				"must be a valid Prometheus duration (e.g., 1m, 30s, 1h)",
			))
		}
	}


	if r.Spec.EvaluationSec != 0 && r.Spec.EvaluationSec < 5 {
		errs = append(errs, field.Invalid(
			field.NewPath("spec").Child("evaluationSec"),
			r.Spec.EvaluationSec,
			"evaluationSec must be >= 5",
		))
	}

	if r.Spec.RunbookURL != "" {
		if _, err := url.ParseRequestURI(r.Spec.RunbookURL); err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("spec").Child("runbookURL"),
				r.Spec.RunbookURL,
				fmt.Sprintf("invalid URL: %v", err),
			))
		}
	}

	if len(errs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "observability.io", Kind: "AlertPolicy"},
			r.Name,
			errs,
		)
	}

	return nil
}