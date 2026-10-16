package correlation

import (
	"context"
	"errors"
	"testing"
)

type testRule struct {
	name, version    string
	priority         int
	enabled, matched bool
	err              error
}

func (r testRule) Name() string    { return r.name }
func (r testRule) Version() string { return r.version }
func (r testRule) Priority() int   { return r.priority }
func (r testRule) Enabled() bool   { return r.enabled }
func (r testRule) Match(context.Context, EvaluationInput) (MatchResult, error) {
	return MatchResult{Matched: r.matched, FingerprintInput: FingerprintInput{ClusterID: "c", RootResourceUID: "u"}}, r.err
}

func TestRegistryFindSnapshotIsolationAndRuleError(t *testing.T) {
	original := []CorrelationRule{testRule{name: "tenant/policy", priority: 1, enabled: true}}
	registry := NewRegistry(original...)
	original[0] = testRule{name: "mutated"}
	if _, ok := registry.Find("policy"); !ok {
		t.Fatal("policy suffix lookup failed or caller mutated registry state")
	}
	snapshot := registry.Snapshot()
	snapshot[0] = testRule{name: "snapshot-mutated"}
	if _, ok := registry.Find("tenant/policy"); !ok {
		t.Fatal("snapshot mutation changed registry state")
	}
	sentinel := errors.New("rule failed")
	_, err := NewRegistry(testRule{name: "broken", priority: 1, enabled: true, err: sentinel}).Evaluate(context.Background(), EvaluationInput{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error=%v", err)
	}
}

func TestRegistryPriorityDisabledAndNoMatch(t *testing.T) {
	r := NewRegistry(
		testRule{name: "low", version: "v1", priority: 1, enabled: true, matched: true},
		testRule{name: "disabled", version: "v1", priority: 100, enabled: false, matched: true},
		testRule{name: "high", version: "v2", priority: 10, enabled: true, matched: true},
	)
	result, err := r.Evaluate(context.Background(), EvaluationInput{PolicyGeneration: 7})
	if err != nil {
		t.Fatal(err)
	}
	if result.RuleName != "high" || result.RuleVersion != "v2" || result.PolicyGeneration != 7 || result.Fingerprint == "" {
		t.Fatalf("result=%#v", result)
	}
	_, err = NewRegistry(testRule{name: "none", priority: 1, enabled: true}).Evaluate(context.Background(), EvaluationInput{})
	if !errors.Is(err, ErrNoMatch) {
		t.Fatalf("error=%v", err)
	}
}

func TestRegistrySamePriorityConflict(t *testing.T) {
	r := NewRegistry(testRule{name: "b", priority: 5, enabled: true, matched: true}, testRule{name: "a", priority: 5, enabled: true, matched: true})
	_, err := r.Evaluate(context.Background(), EvaluationInput{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Priority != 5 || len(conflict.Rules) != 2 || conflict.Rules[0] != "a" {
		t.Fatalf("error=%v", err)
	}
}

func TestFingerprintInputIsDeterministic(t *testing.T) {
	a := FingerprintInput{Rule: "r", RuleVersion: "v1", ClusterID: "c", RootResourceUID: "u", Dimensions: map[string]string{"container": "api", "namespace": "ns"}}
	b := FingerprintInput{Rule: "r", RuleVersion: "v1", ClusterID: "c", RootResourceUID: "u", Dimensions: map[string]string{"namespace": "ns", "container": "api"}}
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("map order changed fingerprint")
	}
}
