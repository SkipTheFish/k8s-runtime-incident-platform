package operator

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/correlation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestCorrelationPolicyReconcileCreateUpdateDisableInvalidAndRestart(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := obsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	policy := controllerPolicy()
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&obsv1.CorrelationPolicy{}).WithObjects(policy).Build()
	registry := correlation.NewRegistry()
	reconciler := &CorrelationPolicyReconciler{Client: client, Scheme: scheme, Registry: registry}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ops", Name: "custom"}}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(registry.Snapshot()) != 4 {
		t.Fatalf("rules=%d", len(registry.Snapshot()))
	}
	var got obsv1.CorrelationPolicy
	if err := client.Get(context.Background(), req.NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.ObservedGeneration != 1 || got.Status.CompiledVersion != "generation-1" || len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Fatalf("status=%#v", got.Status)
	}
	got.Spec.Enabled = false
	got.Generation = 2
	if err := client.Update(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(registry.Snapshot()) != 3 {
		t.Fatalf("disabled rules=%d", len(registry.Snapshot()))
	}
	if err := client.Get(context.Background(), req.NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	got.Spec.Enabled = true
	got.Spec.Window = "broken"
	got.Generation = 3
	if err := client.Update(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(context.Background(), req); err == nil {
		t.Fatal("invalid policy did not fail")
	}
	if err := client.Get(context.Background(), req.NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Conditions[0].Status != metav1.ConditionFalse || got.Status.ObservedGeneration != 3 {
		t.Fatalf("invalid status=%#v", got.Status)
	}
	got.Spec.Window = "10m"
	got.Generation = 4
	if err := client.Update(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	restarted := correlation.NewRegistry()
	if _, err := (&CorrelationPolicyReconciler{Client: client, Scheme: scheme, Registry: restarted}).Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(restarted.Snapshot()) != 4 {
		t.Fatalf("restart rules=%d", len(restarted.Snapshot()))
	}
}

func controllerPolicy() *obsv1.CorrelationPolicy {
	return &obsv1.CorrelationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom", Namespace: "ops", Generation: 1}, Spec: obsv1.CorrelationPolicySpec{Enabled: true, Sources: []obsv1.CorrelationSource{obsv1.CorrelationSourceKubernetes}, EventReasons: []string{"BackOff"}, GroupBy: []string{"rootResourceUID"}, Window: "10m", ReopenWindow: "30m", Severity: obsv1.CorrelationSeverityWarning, Priority: 50}}
}
