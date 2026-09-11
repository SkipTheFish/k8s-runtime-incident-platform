package correlation

import (
	"context"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
	"testing"
)

func TestCrashLoopBackOffRule(t *testing.T) {
	rule := NewCrashLoopBackOffRule(90)
	root := resourcegraph.ResourceKey{UID: "deploy-1", Kind: "Deployment"}
	tests := []struct {
		name              string
		item              signal.Signal
		matched, recovery bool
	}{
		{"event restart", signal.Signal{Source: signal.SourceKubernetes, Reason: "BackOff", Message: "Back-off restarting failed container api", Labels: map[string]string{"container": "api"}}, true, false},
		{"event short", signal.Signal{Source: signal.SourceKubernetes, Reason: "BackOff", Labels: map[string]string{"container": "api"}}, true, false},
		{"prom firing", signal.Signal{Source: signal.SourcePrometheus, Type: signal.TypeAlertFiring, Reason: "KubePodCrashLooping", Labels: map[string]string{"container": "api"}}, true, false},
		{"prom resolved", signal.Signal{Source: signal.SourcePrometheus, Type: signal.TypeAlertResolved, Reason: "KubePodContainerRestarting", Labels: map[string]string{"container": "api"}}, true, true},
		{"image reason", signal.Signal{Source: signal.SourceKubernetes, Reason: "ImagePullBackOff", Message: "Back-off pulling image"}, false, false},
		{"ambiguous pull BackOff", signal.Signal{Source: signal.SourceKubernetes, Reason: "BackOff", Message: "Back-off pulling image example.invalid/app"}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rule.Match(context.Background(), EvaluationInput{Signal: tt.item, Root: root})
			if err != nil {
				t.Fatal(err)
			}
			if got.Matched != tt.matched || got.Recovery != tt.recovery {
				t.Fatalf("result=%#v", got)
			}
		})
	}
}

func TestCrashLoopFingerprintMergeAndSplit(t *testing.T) {
	rule := NewCrashLoopBackOffRule(90)
	fp := func(root, container, pod string) string {
		got, _ := rule.Match(context.Background(), EvaluationInput{Signal: signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Reason: "BackOff", Message: "restarting failed container", Resource: signal.ObjectReference{UID: pod}, Labels: map[string]string{"container": container}}, Root: resourcegraph.ResourceKey{UID: root}})
		return got.FingerprintInput.Fingerprint()
	}
	base := fp("deployment", "api", "pod-a")
	if base != fp("deployment", "api", "pod-b") {
		t.Fatal("same root and container did not merge")
	}
	if base == fp("deployment", "sidecar", "pod-a") {
		t.Fatal("different containers merged")
	}
	if base == fp("other-deployment", "api", "pod-a") {
		t.Fatal("different roots merged")
	}
}
