package operator

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/correlation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CorrelationPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Registry *correlation.Registry
}

func (r *CorrelationPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var target obsv1.CorrelationPolicy
	err := r.Get(ctx, req.NamespacedName, &target)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	var policies obsv1.CorrelationPolicyList
	if err := r.List(ctx, &policies); err != nil {
		return ctrl.Result{}, err
	}
	rules := correlation.BuiltInRules()
	var targetErr error
	for index := range policies.Items {
		policy := &policies.Items[index]
		compiled, compileErr := correlation.CompilePolicy(policy)
		if compileErr != nil {
			if policy.Namespace == req.Namespace && policy.Name == req.Name {
				targetErr = compileErr
			}
			continue
		}
		if policy.Spec.Enabled {
			rules = append(rules, compiled)
		}
	}
	r.Registry.Replace(rules)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	condition := metav1.Condition{Type: "Ready", ObservedGeneration: target.Generation, LastTransitionTime: metav1.Now(), Status: metav1.ConditionTrue, Reason: "Compiled", Message: "policy compiled into runtime snapshot"}
	target.Status.CompiledVersion = fmt.Sprintf("generation-%d", target.Generation)
	if targetErr != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "CompileFailed"
		condition.Message = targetErr.Error()
		target.Status.CompiledVersion = ""
	}
	target.Status.ObservedGeneration = target.Generation
	meta.SetStatusCondition(&target.Status.Conditions, condition)
	if statusErr := r.Status().Update(ctx, &target); statusErr != nil {
		return ctrl.Result{}, statusErr
	}
	return ctrl.Result{}, targetErr
}

func (r *CorrelationPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&obsv1.CorrelationPolicy{}).Complete(r)
}
