package operator

import (
	"context"

	obsv1 "observability-platform/api/v1"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObservabilityTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ObservabilityTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var target obsv1.ObservabilityTarget

	if err := r.Get(ctx, req.NamespacedName, &target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	sm := BuildServiceMonitor(&target)

	if err := ctrl.SetControllerReference(&target, sm, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	var existing monitoringv1.ServiceMonitor

	err := r.Get(ctx, client.ObjectKey{
		Namespace: sm.Namespace,
		Name:      sm.Name,
	}, &existing)

	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, r.Create(ctx, sm)
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	existing.Spec = sm.Spec
	existing.Labels = sm.Labels

	return ctrl.Result{}, r.Update(ctx, &existing)
}

func (r *ObservabilityTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&obsv1.ObservabilityTarget{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		Complete(r)
}