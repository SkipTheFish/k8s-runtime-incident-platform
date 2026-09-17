package v1

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"strings"
	"testing"
)

func validCorrelationPolicy() *CorrelationPolicy {
	return &CorrelationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "candidate", Namespace: "ops"}, Spec: CorrelationPolicySpec{Enabled: true, Sources: []CorrelationSource{CorrelationSourceKubernetes}, EventReasons: []string{"BackOff"}, Namespaces: []string{"apps"}, GroupBy: []string{"rootResourceUID", "container"}, Window: "10m", ReopenWindow: "30m", Severity: CorrelationSeverityWarning, Priority: 100, FingerprintTemplate: "{{.RootResourceUID}}/{{.container}}"}}
}

func TestCorrelationPolicyValidatorRejectionsAndLegalPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	existing := validCorrelationPolicy()
	existing.Name = "existing"
	validator := &correlationPolicyValidator{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()}
	tests := []struct {
		name   string
		mutate func(*CorrelationPolicy)
		want   string
	}{
		{"empty groupBy", func(p *CorrelationPolicy) { p.Spec.GroupBy = nil }, "spec.groupBy"},
		{"window out of range", func(p *CorrelationPolicy) { p.Spec.Window = "25h" }, "spec.window"},
		{"unsupported groupBy", func(p *CorrelationPolicy) { p.Spec.GroupBy = []string{"podIP"} }, "Unsupported value"},
		{"template compile", func(p *CorrelationPolicy) { p.Spec.FingerprintTemplate = "{{" }, "template cannot be compiled"},
		{"same priority overlap", func(p *CorrelationPolicy) {}, "existing"},
		{"legal distinct priority", func(p *CorrelationPolicy) { p.Spec.Priority = 101 }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := validCorrelationPolicy()
			tt.mutate(policy)
			_, err := validator.ValidateCreate(context.Background(), policy)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), policy.Spec.FingerprintTemplate) && tt.name == "template compile" {
				t.Fatal("template content leaked")
			}
		})
	}
}

func TestCorrelationPolicyOverlapUsesSourceReasonAndNamespace(t *testing.T) {
	a := validCorrelationPolicy().Spec
	b := a
	if !policyDomainsOverlap(a, b) {
		t.Fatal("identical domains should overlap")
	}
	b.Sources = []CorrelationSource{CorrelationSourcePrometheus}
	if policyDomainsOverlap(a, b) {
		t.Fatal("different sources overlap")
	}
	b = a
	b.EventReasons = []string{"FailedScheduling"}
	if policyDomainsOverlap(a, b) {
		t.Fatal("different reasons overlap")
	}
	b = a
	b.Namespaces = nil
	if !policyDomainsOverlap(a, b) {
		t.Fatal("empty namespaces should be wildcard")
	}
}
