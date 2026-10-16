package correlation

import (
	"context"
	"testing"

	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

func TestImagePullBackOffRule(t *testing.T) {
	rule := NewImagePullBackOffRule(100)
	root := resourcegraph.ResourceKey{ClusterID: "c", APIVersion: "apps/v1", Kind: "Deployment", Namespace: "ns", Name: "api", UID: "deploy-1"}
	tests := []struct {
		name                      string
		item                      signal.Signal
		wantMatched, wantRecovery bool
	}{
		{"kubernetes ImagePullBackOff", signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Type: signal.TypeWarning, Reason: "ImagePullBackOff", Labels: map[string]string{"container": "api"}}, true, false},
		{"kubernetes ErrImagePull", signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Type: signal.TypeWarning, Reason: "ErrImagePull", Labels: map[string]string{"container": "api"}}, true, false},
		{"kubernetes Failed pull", signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Type: signal.TypeWarning, Reason: "Failed", Message: "Failed to pull image invalid.example/app", Labels: map[string]string{"container": "api"}}, true, false},
		{"kubernetes BackOff pull", signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Type: signal.TypeWarning, Reason: "BackOff", Message: "Back-off pulling image invalid.example/app", Labels: map[string]string{"container": "api"}}, true, false},
		{"prometheus firing", signal.Signal{ClusterID: "c", Source: signal.SourcePrometheus, Type: signal.TypeAlertFiring, Reason: "KubePodImagePullBackOff", Labels: map[string]string{"container": "api"}}, true, false},
		{"prometheus resolved", signal.Signal{ClusterID: "c", Source: signal.SourcePrometheus, Type: signal.TypeAlertResolved, Reason: "KubePodImagePullBackOff", Labels: map[string]string{"container": "api"}}, true, true},
		{"other", signal.Signal{Source: signal.SourceKubernetes, Reason: "BackOff", Message: "Back-off restarting failed container"}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rule.Match(context.Background(), EvaluationInput{Signal: tt.item, Root: root})
			if err != nil {
				t.Fatal(err)
			}
			if got.Matched != tt.wantMatched || got.Recovery != tt.wantRecovery {
				t.Fatalf("result=%#v", got)
			}
			if got.Matched && (got.FingerprintInput.RootResourceUID != "deploy-1" || got.Explain.Reason == "") {
				t.Fatalf("result=%#v", got)
			}
		})
	}
}

func TestImagePullFingerprintMergeAndSplit(t *testing.T) {
	rule := NewImagePullBackOffRule(100)
	match := func(rootUID, container, pod string) string {
		got, err := rule.Match(context.Background(), EvaluationInput{Signal: signal.Signal{ClusterID: "c", Source: signal.SourceKubernetes, Reason: "ImagePullBackOff", Resource: signal.ObjectReference{UID: pod}, Labels: map[string]string{"container": container}}, Root: resourcegraph.ResourceKey{UID: rootUID}})
		if err != nil {
			t.Fatal(err)
		}
		return got.FingerprintInput.Fingerprint()
	}
	first := match("deploy-1", "api", "pod-a")
	if second := match("deploy-1", "api", "pod-b"); first != second {
		t.Fatal("pods under same root/container did not merge")
	}
	if other := match("deploy-1", "sidecar", "pod-a"); first == other {
		t.Fatal("different containers were merged")
	}
	if other := match("deploy-2", "api", "pod-a"); first == other {
		t.Fatal("different roots were merged")
	}
}
