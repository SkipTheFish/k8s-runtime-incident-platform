package correlation

import (
	"context"
	"testing"

	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

var benchmarkMatchResult MatchResult

func benchmarkEvaluationInput() EvaluationInput {
	return EvaluationInput{
		Signal: signal.Signal{
			ClusterID: "benchmark-cluster", Source: signal.SourceKubernetes,
			Type: signal.TypeWarning, Reason: "BackOff",
			Message:  "Back-off restarting failed container api",
			Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "benchmark", Name: "api-abc", UID: "pod-uid"},
			Labels:   map[string]string{"container": "api"},
		},
		Root: resourcegraph.ResourceKey{ClusterID: "benchmark-cluster", APIVersion: "apps/v1", Kind: "Deployment", Namespace: "benchmark", Name: "api", UID: "deployment-uid"},
	}
}

func BenchmarkRuleEvaluation(b *testing.B) {
	rule := NewCrashLoopBackOffRule(90)
	input := benchmarkEvaluationInput()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		result, err := rule.Match(ctx, input)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMatchResult = result
	}
}

func BenchmarkIncidentCorrelation(b *testing.B) {
	registry := NewRegistry(NewImagePullBackOffRule(100), NewCrashLoopBackOffRule(90), NewFailedSchedulingRule(80))
	input := benchmarkEvaluationInput()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		result, err := registry.Evaluate(ctx, input)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMatchResult = result
	}
}
