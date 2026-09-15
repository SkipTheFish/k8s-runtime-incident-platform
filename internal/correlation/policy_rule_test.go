package correlation

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
	"testing"
)

func TestCompilePolicyCreatesDeterministicGenerationRule(t *testing.T) {
	policy := &obsv1.CorrelationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom", Namespace: "ops", Generation: 4}, Spec: obsv1.CorrelationPolicySpec{Enabled: true, Sources: []obsv1.CorrelationSource{obsv1.CorrelationSourceKubernetes}, EventReasons: []string{"BackOff"}, Namespaces: []string{"apps"}, GroupBy: []string{"rootResourceUID", "container"}, Window: "5m", ReopenWindow: "20m", Severity: obsv1.CorrelationSeverityCritical, Priority: 50, Explain: true}}
	rule, err := CompilePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	if rule.Name() != "ops/custom" || rule.Version() != "generation-4" || rule.Priority() != 50 {
		t.Fatalf("rule=%s %s %d", rule.Name(), rule.Version(), rule.Priority())
	}
	input := EvaluationInput{PolicyGeneration: 4, Signal: signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Reason: "BackOff", Namespace: "apps", Labels: map[string]string{"container": "api"}}, Root: resourcegraph.ResourceKey{UID: "deploy"}}
	first, err := NewRegistry(rule).Evaluate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRegistry(rule).Evaluate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint || first.PolicyGeneration != 4 || first.RuleVersion != "generation-4" {
		t.Fatalf("results=%#v %#v", first, second)
	}
	input.Signal.Namespace = "other"
	if _, err := NewRegistry(rule).Evaluate(context.Background(), input); err != ErrNoMatch {
		t.Fatalf("error=%v", err)
	}
}
