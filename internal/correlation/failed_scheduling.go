package correlation

import (
	"context"
	"regexp"
	"strings"
	"time"

	"observability-platform/internal/resourcegraph"
)

var schedulingSecret = regexp.MustCompile(`(?i)\b(token|password|secret|authorization)\s*[:=]\s*[^\s,;]+`)

type FailedSchedulingRule struct {
	RulePriority int
	RuleEnabled  bool
}

func NewFailedSchedulingRule(priority int) FailedSchedulingRule {
	return FailedSchedulingRule{RulePriority: priority, RuleEnabled: true}
}
func (r FailedSchedulingRule) Name() string    { return "failed-scheduling" }
func (r FailedSchedulingRule) Version() string { return "v1" }
func (r FailedSchedulingRule) Priority() int   { return r.RulePriority }
func (r FailedSchedulingRule) Enabled() bool   { return r.RuleEnabled }

func (r FailedSchedulingRule) Match(_ context.Context, input EvaluationInput) (MatchResult, error) {
	reason := strings.ToLower(strings.TrimSpace(input.Signal.Reason))
	if reason != "failedscheduling" && reason != "scheduled" {
		return MatchResult{}, nil
	}
	recovery := reason == "scheduled"
	root := input.Root
	rootFallback := false
	if root.UID == "" {
		root = resourcegraph.ResourceKey{ClusterID: input.Signal.ClusterID, APIVersion: input.Signal.Resource.APIVersion, Kind: input.Signal.Resource.Kind, Namespace: input.Signal.Resource.Namespace, Name: input.Signal.Resource.Name, UID: input.Signal.Resource.UID}
		rootFallback = true
	}
	fp := FingerprintInput{Rule: r.Name(), RuleVersion: r.Version(), ClusterID: input.Signal.ClusterID, RootResourceUID: root.UID, Dimensions: map[string]string{"podUID": input.Signal.Resource.UID}}
	conditions := []string{"signal reason is FailedScheduling", "fingerprint includes Pod UID to isolate pending Pods"}
	if rootFallback {
		conditions = append(conditions, "owner unresolved; Pod identity is used as root")
	}
	if recovery {
		conditions = []string{"signal reason is Scheduled", "Pod UID matches the scheduling failure"}
	}
	return MatchResult{Matched: true, Recovery: recovery, Root: root, FingerprintInput: fp, Window: 15 * time.Minute, ReopenWindow: 15 * time.Minute, Severity: "warning", Evidence: []Evidence{{Kind: "scheduler", Summary: redactSchedulingSummary(input.Signal.Message), Attributes: map[string]string{"reason": input.Signal.Reason, "podUID": input.Signal.Resource.UID}}}, Explain: Explain{MatchedConditions: conditions, Reason: "scheduling failures are grouped only for the same Pod identity; Scheduled is the recovery condition"}}, nil
}

func redactSchedulingSummary(message string) string {
	message = schedulingSecret.ReplaceAllString(message, "$1=[REDACTED]")
	return boundedSummary(message, 512)
}
