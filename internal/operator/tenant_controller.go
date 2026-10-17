package operator

import (
	"context"

	obsv1 "observability-platform/api/v1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var tenant obsv1.ObserveTenant

	if err := r.Get(ctx, req.NamespacedName, &tenant); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	nsName := "obs-tenant-" + tenant.Spec.TenantID

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"observability.io/tenant": tenant.Spec.TenantID,
				"observability.io/owner":  tenant.Spec.Owner,
			},
		},
	}

	var existingNS corev1.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: nsName}, &existingNS); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, ns); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	role := buildTenantRole(nsName)

	var existingRole rbacv1.Role

	if err := r.Get(ctx, client.ObjectKey{Name: role.Name, Namespace: role.Namespace}, &existingRole); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, role); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	} else {
		existingRole.Rules = role.Rules
		if err := r.Update(ctx, &existingRole); err != nil {
			return ctrl.Result{}, err
		}
	}
	tenant.Status.NamespaceCreated = true
	tenant.Status.ObservedGeneration = tenant.Generation
	if err := r.Status().Update(ctx, &tenant); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func buildTenantRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "observability-tenant-admin",
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"observability.io"},
				Resources: []string{
					"alertpolicies",
					"observabilitytargets",
				},
				Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{
					"pods",
					"services",
					"endpoints",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}
}

func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&obsv1.ObserveTenant{}).
		Owns(&rbacv1.Role{}).
		Complete(r)
}
