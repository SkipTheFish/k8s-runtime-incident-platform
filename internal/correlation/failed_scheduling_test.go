package correlation

import (
	"context"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
	"strings"
	"testing"
)

func TestFailedSchedulingRule(t *testing.T) {
	rule := NewFailedSchedulingRule(80)
	base := signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Reason: "FailedScheduling", Message: "0/3 nodes available; token=very-secret password:guess", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "pending", UID: "pod-1"}}
	result, err := rule.Match(context.Background(), EvaluationInput{Signal: base, Root: resourcegraph.ResourceKey{UID: "deploy-1", Kind: "Deployment"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Matched || result.Recovery || result.FingerprintInput.Dimensions["podUID"] != "pod-1" {
		t.Fatalf("result=%#v", result)
	}
	summary := result.Evidence[0].Summary
	if strings.Contains(summary, "very-secret") || strings.Contains(summary, "guess") || !strings.Contains(summary, "[REDACTED]") {
		t.Fatalf("summary=%q", summary)
	}
	long := base
	long.Message = strings.Repeat("x", 600)
	got, _ := rule.Match(context.Background(), EvaluationInput{Signal: long})
	if len(got.Evidence[0].Summary) != 512 {
		t.Fatalf("summary length=%d", len(got.Evidence[0].Summary))
	}
}

func TestFailedSchedulingMergeSplitRecoveryAndOwnerFallback(t *testing.T) {
	rule := NewFailedSchedulingRule(80)
	fp := func(uid, event string) string {
		got, _ := rule.Match(context.Background(), EvaluationInput{Signal: signal.Signal{ClusterID: "c", Reason: "FailedScheduling", SourceEventKey: event, Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "p", UID: uid}}, Root: resourcegraph.ResourceKey{UID: "deployment"}})
		return got.FingerprintInput.Fingerprint()
	}
	if fp("pod-1", "event-a") != fp("pod-1", "event-b") {
		t.Fatal("repeat events for one Pod did not aggregate")
	}
	if fp("pod-1", "event-a") == fp("pod-2", "event-a") {
		t.Fatal("different pending Pods were merged")
	}
	failure := signal.Signal{ClusterID: "c", Reason: "FailedScheduling", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "p", UID: "pod-1"}}
	fallback, _ := rule.Match(context.Background(), EvaluationInput{Signal: failure})
	if fallback.Root.UID != "pod-1" || fallback.Root.Kind != "Pod" {
		t.Fatalf("fallback=%#v", fallback.Root)
	}
	recovered := failure
	recovered.Reason = "Scheduled"
	result, _ := rule.Match(context.Background(), EvaluationInput{Signal: recovered})
	if !result.Matched || !result.Recovery || result.FingerprintInput.Fingerprint() != fallback.FingerprintInput.Fingerprint() {
		t.Fatalf("recovery=%#v", result)
	}
}
