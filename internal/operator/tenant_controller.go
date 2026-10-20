package operator

import (
	"context"
	"reflect"

	obsv1 "observability-platform/api/v1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	tenantSourceNamespaceLabel = "observability.io/tenant-source-namespace"
	tenantSourceNameLabel      = "observability.io/tenant-source-name"
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
			Name:   nsName,
			Labels: managedTenantLabels(&tenant),
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
	} else if mergeManagedLabels(&existingNS.ObjectMeta, ns.Labels) {
		if err := r.Update(ctx, &existingNS); err != nil {
			return ctrl.Result{}, err
		}
	}

	role := buildTenantRole(nsName, managedTenantLabels(&tenant))

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
		changed := mergeManagedLabels(&existingRole.ObjectMeta, role.Labels)
		if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
			existingRole.Rules = role.Rules
			changed = true
		}
		if changed {
			if err := r.Update(ctx, &existingRole); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	if !tenant.Status.NamespaceCreated || tenant.Status.ObservedGeneration != tenant.Generation {
		tenant.Status.NamespaceCreated = true
		tenant.Status.ObservedGeneration = tenant.Generation
		if err := r.Status().Update(ctx, &tenant); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func buildTenantRole(namespace string, labels map[string]string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "observability-tenant-admin",
			Namespace: namespace,
			Labels:    labels,
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

func managedTenantLabels(tenant *obsv1.ObserveTenant) map[string]string {
	return map[string]string{
		"observability.io/tenant":  tenant.Spec.TenantID,
		"observability.io/owner":   tenant.Spec.Owner,
		tenantSourceNamespaceLabel: tenant.Namespace,
		tenantSourceNameLabel:      tenant.Name,
	}
}

func mergeManagedLabels(meta *metav1.ObjectMeta, desired map[string]string) bool {
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	changed := false
	for key, value := range desired {
		if meta.Labels[key] != value {
			meta.Labels[key] = value
			changed = true
		}
	}
	return changed
}

func mapTenantManagedResource(_ context.Context, object client.Object) []reconcile.Request {
	labels := object.GetLabels()
	namespace, name := labels[tenantSourceNamespaceLabel], labels[tenantSourceNameLabel]
	if namespace == "" || name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: namespace, Name: name}}}
}

func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&obsv1.ObserveTenant{}).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(mapTenantManagedResource)).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(mapTenantManagedResource)).
		Complete(r)
}
