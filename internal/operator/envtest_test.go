package operator

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/correlation"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type envtestRepository struct{}

func (envtestRepository) UpsertAlertPolicy(context.Context, *obsv1.AlertPolicy) (int64, error) {
	return 42, nil
}
func (envtestRepository) DeleteAlertPolicy(context.Context, string, string) error { return nil }
func (envtestRepository) GetAlertPolicyID(context.Context, string, string) (int64, bool, error) {
	return 42, true, nil
}

func TestOperatorReconcilersWithEnvtest(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("controller-runtime envtest cannot stop control-plane processes reliably on Windows; run this gate on Linux")
	}
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("KUBEBUILDER_ASSETS is not set; run make envtest-assets or make test-envtest")
	}
	scheme := runtime.NewScheme()
	for name, add := range map[string]func(*runtime.Scheme) error{
		"observability": obsv1.AddToScheme,
		"monitoring":    monitoringv1.AddToScheme,
		"core":          corev1.AddToScheme,
		"rbac":          rbacv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("add %s scheme: %v", name, err)
		}
	}
	testEnvironment := &envtest.Environment{CRDDirectoryPaths: []string{
		filepath.Join("..", "..", "deploy", "helm", "runtime-incident-platform", "crds"),
		filepath.Join("testdata", "crds"),
	}, ErrorIfCRDPathMissing: true}
	config, err := testEnvironment.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := testEnvironment.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, namespace := range []string{"policies", "targets"} {
		if err := kubeClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("AlertPolicy creates owned PrometheusRule and status", func(t *testing.T) {
		enabled := true
		policy := &obsv1.AlertPolicy{ObjectMeta: metav1.ObjectMeta{Name: "latency", Namespace: "policies"}, Spec: obsv1.AlertPolicySpec{TenantID: "tenant-a", PolicyName: "HighLatency", Query: "up", Threshold: 0, Operator: "==", Duration: "1m", Severity: obsv1.SeverityCritical, Enabled: &enabled}}
		if err := kubeClient.Create(ctx, policy); err != nil {
			t.Fatal(err)
		}
		reconciler := &AlertPolicyReconciler{Client: kubeClient, Scheme: scheme, Repo: envtestRepository{}}
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}}
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
		var stored obsv1.AlertPolicy
		if err := kubeClient.Get(ctx, request.NamespacedName, &stored); err != nil {
			t.Fatal(err)
		}
		if !containsString(stored.Finalizers, AlertPolicyFinalizer) || !stored.Status.Synced || stored.Status.DBID != 42 || stored.Status.ObservedGeneration != stored.Generation {
			t.Fatalf("policy lifecycle metadata=%+v status=%+v", stored.Finalizers, stored.Status)
		}
		var generated monitoringv1.PrometheusRule
		if err := kubeClient.Get(ctx, types.NamespacedName{Name: "alertpolicy-latency", Namespace: "policies"}, &generated); err != nil {
			t.Fatal(err)
		}
		assertControllerOwner(t, generated.OwnerReferences, policy.Name)
	})

	t.Run("ObservabilityTarget creates owned ServiceMonitor and status", func(t *testing.T) {
		target := &obsv1.ObservabilityTarget{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "targets"}, Spec: obsv1.ObservabilityTargetSpec{TenantID: "tenant-a", ServiceName: "api", Namespace: "workload", Selector: map[string]string{"app": "api"}, PortName: "metrics", ScrapePath: "/metrics", ScrapeInterval: "30s"}}
		if err := kubeClient.Create(ctx, target); err != nil {
			t.Fatal(err)
		}
		reconciler := &ObservabilityTargetReconciler{Client: kubeClient, Scheme: scheme}
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: target.Name, Namespace: target.Namespace}}
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
		var stored obsv1.ObservabilityTarget
		if err := kubeClient.Get(ctx, request.NamespacedName, &stored); err != nil {
			t.Fatal(err)
		}
		if !stored.Status.Synced || stored.Status.ObservedGeneration != stored.Generation {
			t.Fatalf("target status=%+v generation=%d", stored.Status, stored.Generation)
		}
		var generated monitoringv1.ServiceMonitor
		if err := kubeClient.Get(ctx, types.NamespacedName{Name: "target-api", Namespace: "targets"}, &generated); err != nil {
			t.Fatal(err)
		}
		assertControllerOwner(t, generated.OwnerReferences, target.Name)
	})

	t.Run("ObserveTenant creates namespace and role with observed generation", func(t *testing.T) {
		tenant := &obsv1.ObserveTenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a", Namespace: "policies"}, Spec: obsv1.ObserveTenantSpec{TenantID: "tenant-a", DisplayName: "Tenant A", Owner: "platform", Enabled: true}}
		if err := kubeClient.Create(ctx, tenant); err != nil {
			t.Fatal(err)
		}
		reconciler := &TenantReconciler{Client: kubeClient, Scheme: scheme}
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: tenant.Name, Namespace: tenant.Namespace}}
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
		var namespace corev1.Namespace
		if err := kubeClient.Get(ctx, types.NamespacedName{Name: "obs-tenant-tenant-a"}, &namespace); err != nil {
			t.Fatal(err)
		}
		var role rbacv1.Role
		if err := kubeClient.Get(ctx, types.NamespacedName{Name: "observability-tenant-admin", Namespace: namespace.Name}, &role); err != nil {
			t.Fatal(err)
		}
		var stored obsv1.ObserveTenant
		if err := kubeClient.Get(ctx, request.NamespacedName, &stored); err != nil {
			t.Fatal(err)
		}
		if !stored.Status.NamespaceCreated || stored.Status.ObservedGeneration != stored.Generation {
			t.Fatalf("tenant status=%+v generation=%d", stored.Status, stored.Generation)
		}
	})

	t.Run("CorrelationPolicy records compiled generation", func(t *testing.T) {
		policy := &obsv1.CorrelationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom", Namespace: "policies"}, Spec: obsv1.CorrelationPolicySpec{Enabled: true, Sources: []obsv1.CorrelationSource{obsv1.CorrelationSourceKubernetes}, EventReasons: []string{"BackOff"}, GroupBy: []string{"rootResourceUID"}, Window: "5m", ReopenWindow: "30m", Severity: obsv1.CorrelationSeverityCritical, Priority: 10}}
		if err := kubeClient.Create(ctx, policy); err != nil {
			t.Fatal(err)
		}
		registry := correlation.NewRegistry()
		reconciler := &CorrelationPolicyReconciler{Client: kubeClient, Scheme: scheme, Registry: registry}
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}}
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
		var stored obsv1.CorrelationPolicy
		if err := kubeClient.Get(ctx, request.NamespacedName, &stored); err != nil {
			t.Fatal(err)
		}
		if stored.Status.ObservedGeneration != stored.Generation || stored.Status.CompiledVersion == "" || len(stored.Status.Conditions) != 1 {
			t.Fatalf("correlation status=%+v generation=%d", stored.Status, stored.Generation)
		}
		if _, ok := registry.Find("policies/custom"); !ok {
			t.Fatal("compiled rule not published to registry")
		}
	})
}

func assertControllerOwner(t *testing.T, owners []metav1.OwnerReference, name string) {
	t.Helper()
	if len(owners) != 1 || owners[0].Name != name || owners[0].Controller == nil || !*owners[0].Controller {
		t.Fatalf("controller owner=%+v want=%s", owners, name)
	}
}
