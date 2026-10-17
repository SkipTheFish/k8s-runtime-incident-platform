package correlation

import (
	"context"
	"strings"
	"time"

	"observability-platform/internal/signal"
)

type ImagePullBackOffRule struct {
	RulePriority int
	RuleEnabled  bool
}

func NewImagePullBackOffRule(priority int) ImagePullBackOffRule {
	return ImagePullBackOffRule{RulePriority: priority, RuleEnabled: true}
}

func (r ImagePullBackOffRule) Name() string    { return "image-pull-backoff" }
func (r ImagePullBackOffRule) Version() string { return "v1" }
func (r ImagePullBackOffRule) Priority() int   { return r.RulePriority }
func (r ImagePullBackOffRule) Enabled() bool   { return r.RuleEnabled }

func (r ImagePullBackOffRule) Match(_ context.Context, input EvaluationInput) (MatchResult, error) {
	if !isImagePullSignal(input.Signal) {
		return MatchResult{}, nil
	}
	container := signalDimension(input.Signal, "container", "unknown")
	fingerprintInput := FingerprintInput{
		Rule: r.Name(), RuleVersion: r.Version(), ClusterID: input.Signal.ClusterID,
		RootResourceUID: input.Root.UID, Dimensions: map[string]string{"container": container},
	}
	recovery := (input.Signal.Source == signal.SourcePrometheus && input.Signal.Type == signal.TypeAlertResolved) ||
		(input.Signal.Source == signal.SourceKubernetes && strings.EqualFold(input.Signal.Reason, "Pulled"))
	conditions := []string{"reason identifies an image pull failure", "root resource UID is resolved", "container dimension is stable"}
	if recovery {
		conditions = append(conditions, "Prometheus alert is resolved")
	}
	return MatchResult{
		Matched: true, Recovery: recovery, Root: input.Root, FingerprintInput: fingerprintInput,
		Window: 10 * time.Minute, ReopenWindow: 30 * time.Minute, Severity: "warning",
		Evidence: []Evidence{{Kind: "signal", Summary: boundedSummary(input.Signal.Message, 512), Attributes: map[string]string{"reason": input.Signal.Reason, "container": container, "source": string(input.Signal.Source)}}},
		Explain:  Explain{MatchedConditions: conditions, Reason: "image pull failures are grouped by stable root resource and container"},
	}, nil
}

func isImagePullSignal(item signal.Signal) bool {
	reason := strings.ToLower(item.Reason)
	message := strings.ToLower(item.Message)
	if item.Source == signal.SourceKubernetes && item.Type == signal.TypeNormal && reason == "pulled" {
		return true
	}
	if reason == "imagepullbackoff" || reason == "errimagepull" || strings.Contains(reason, "imagepull") {
		return true
	}
	if item.Source != signal.SourceKubernetes || (reason != "failed" && reason != "backoff") {
		return false
	}
	return strings.Contains(message, "pull image") || strings.Contains(message, "pulling image") || strings.Contains(message, "image pull")
}

func signalDimension(item signal.Signal, key, fallback string) string {
	if value := strings.TrimSpace(item.Labels[key]); value != "" {
		return value
	}
	if value := strings.TrimSpace(item.Resource.Labels[key]); value != "" {
		return value
	}
	return fallback
}

func boundedSummary(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
