package resourcegraph

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResourceKeyStableKeyUsesCompleteIdentity(t *testing.T) {
	base := ResourceKey{ClusterID: "cluster-a", APIVersion: "apps/v1", Kind: "Deployment", Namespace: "payments", Name: "api", UID: "uid-old"}
	first, err := base.StableKey()
	if err != nil {
		t.Fatal(err)
	}
	rebuilt := base
	rebuilt.UID = "uid-new"
	second, err := rebuilt.StableKey()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("same-name resource recreation reused the old key")
	}
	if again, _ := base.StableKey(); again != first {
		t.Fatal("stable key is not deterministic")
	}
}

func TestResourceKeyRequiresFullIdentity(t *testing.T) {
	valid := ResourceKey{ClusterID: "c", APIVersion: "v1", Kind: "Node", Name: "node-a", UID: "u"}
	tests := map[string]func(*ResourceKey){
		"cluster": func(k *ResourceKey) { k.ClusterID = "" },
		"version": func(k *ResourceKey) { k.APIVersion = "" },
		"kind":    func(k *ResourceKey) { k.Kind = "" },
		"name":    func(k *ResourceKey) { k.Name = "" },
		"uid":     func(k *ResourceKey) { k.UID = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if _, err := candidate.StableKey(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestResourceGraphTypesJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 3, 23, 0, 0, 0, time.UTC)
	want := RootResolutionResult{
		Root: ResourceNode{Key: ResourceKey{ClusterID: "c", APIVersion: "apps/v1", Kind: "Deployment", Namespace: "ns", Name: "api", UID: "d1"}, Labels: map[string]string{"app": "api"}, ObservedAt: now},
		Path: []ResourceEdge{{ClusterID: "c", FromUID: "p1", ToUID: "d1", Type: EdgeOwnedBy, ObservedAt: now}},
	}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got RootResolutionResult
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Root.Key.UID != want.Root.Key.UID || got.Path[0].Type != EdgeOwnedBy || !got.Root.ObservedAt.Equal(now) {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}
