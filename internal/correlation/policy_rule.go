package correlation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/signal"
)

type PolicyRule struct {
	name, version        string
	priority             int
	enabled              bool
	sources              map[signal.Source]struct{}
	reasons, namespaces  map[string]struct{}
	groupBy              []string
	window, reopenWindow time.Duration
	severity             string
	explain              bool
}

func CompilePolicy(policy *obsv1.CorrelationPolicy) (CorrelationRule, error) {
	window, err := time.ParseDuration(policy.Spec.Window)
	if err != nil {
		return nil, fmt.Errorf("spec.window: %w", err)
	}
	reopen, err := time.ParseDuration(policy.Spec.ReopenWindow)
	if err != nil {
		return nil, fmt.Errorf("spec.reopenWindow: %w", err)
	}
	if len(policy.Spec.GroupBy) == 0 {
		return nil, fmt.Errorf("spec.groupBy must not be empty")
	}
	r := &PolicyRule{name: policy.Namespace + "/" + policy.Name, version: "generation-" + strconv.FormatInt(policy.Generation, 10), priority: int(policy.Spec.Priority), enabled: policy.Spec.Enabled, groupBy: append([]string(nil), policy.Spec.GroupBy...), window: window, reopenWindow: reopen, severity: string(policy.Spec.Severity), explain: policy.Spec.Explain, sources: map[signal.Source]struct{}{}, reasons: map[string]struct{}{}, namespaces: map[string]struct{}{}}
	for _, source := range policy.Spec.Sources {
		switch source {
		case obsv1.CorrelationSourceKubernetes:
			r.sources[signal.SourceKubernetes] = struct{}{}
		case obsv1.CorrelationSourcePrometheus:
			r.sources[signal.SourcePrometheus] = struct{}{}
		default:
			return nil, fmt.Errorf("spec.sources: unsupported source %q", source)
		}
	}
	for _, reason := range policy.Spec.EventReasons {
		r.reasons[strings.ToLower(reason)] = struct{}{}
	}
	for _, namespace := range policy.Spec.Namespaces {
		r.namespaces[namespace] = struct{}{}
	}
	return r, nil
}

func (r *PolicyRule) Name() string    { return r.name }
func (r *PolicyRule) Version() string { return r.version }
func (r *PolicyRule) Priority() int   { return r.priority }
func (r *PolicyRule) Enabled() bool   { return r.enabled }
func (r *PolicyRule) Match(_ context.Context, input EvaluationInput) (MatchResult, error) {
	if _, ok := r.sources[input.Signal.Source]; !ok {
		return MatchResult{}, nil
	}
	if len(r.reasons) > 0 {
		if _, ok := r.reasons[strings.ToLower(input.Signal.Reason)]; !ok {
			return MatchResult{}, nil
		}
	}
	if len(r.namespaces) > 0 {
		if _, ok := r.namespaces[input.Signal.Namespace]; !ok {
			return MatchResult{}, nil
		}
	}
	dimensions := map[string]string{}
	for _, field := range r.groupBy {
		switch field {
		case "rootResourceUID":
			dimensions[field] = input.Root.UID
		case "container":
			dimensions[field] = signalDimension(input.Signal, "container", "")
		case "podUID":
			dimensions[field] = input.Signal.Resource.UID
		case "namespace":
			dimensions[field] = input.Signal.Namespace
		case "reason":
			dimensions[field] = input.Signal.Reason
		default:
			return MatchResult{}, fmt.Errorf("unsupported groupBy field %q", field)
		}
	}
	fp := FingerprintInput{Rule: r.name, RuleVersion: r.version, ClusterID: input.Signal.ClusterID, RootResourceUID: input.Root.UID, Dimensions: dimensions}
	explain := Explain{Reason: "signal matched compiled CorrelationPolicy"}
	if r.explain {
		explain.MatchedConditions = []string{"source matched", "event reason matched", "namespace matched", "groupBy fields resolved"}
	}
	return MatchResult{Matched: true, Recovery: input.Signal.Type == signal.TypeAlertResolved, Root: input.Root, FingerprintInput: fp, Window: r.window, ReopenWindow: r.reopenWindow, Severity: r.severity, Explain: explain}, nil
}

func BuiltInRules() []CorrelationRule {
	return []CorrelationRule{NewImagePullBackOffRule(300), NewCrashLoopBackOffRule(200), NewFailedSchedulingRule(100)}
}
