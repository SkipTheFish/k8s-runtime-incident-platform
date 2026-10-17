package correlation

import (
	"context"
	"strings"
	"time"

	"observability-platform/internal/signal"
)

type CrashLoopBackOffRule struct {
	RulePriority int
	RuleEnabled  bool
}

func NewCrashLoopBackOffRule(priority int) CrashLoopBackOffRule {
	return CrashLoopBackOffRule{RulePriority: priority, RuleEnabled: true}
}
func (r CrashLoopBackOffRule) Name() string    { return "crash-loop-backoff" }
func (r CrashLoopBackOffRule) Version() string { return "v1" }
func (r CrashLoopBackOffRule) Priority() int   { return r.RulePriority }
func (r CrashLoopBackOffRule) Enabled() bool   { return r.RuleEnabled }

func (r CrashLoopBackOffRule) Match(_ context.Context, input EvaluationInput) (MatchResult, error) {
	if !isCrashLoopSignal(input.Signal) {
		return MatchResult{}, nil
	}
	container := signalDimension(input.Signal, "container", "unknown")
	fp := FingerprintInput{Rule: r.Name(), RuleVersion: r.Version(), ClusterID: input.Signal.ClusterID, RootResourceUID: input.Root.UID, Dimensions: map[string]string{"container": container}}
	recovery := (input.Signal.Source == signal.SourcePrometheus && input.Signal.Type == signal.TypeAlertResolved) ||
		(input.Signal.Source == signal.SourceKubernetes && input.Signal.Type == signal.TypeNormal && strings.EqualFold(input.Signal.Reason, "Started"))
	conditions := []string{"signal identifies container restart backoff", "image pull backoff indicators are absent", "root resource UID is resolved"}
	if recovery {
		conditions = append(conditions, "Prometheus alert is resolved")
	}
	return MatchResult{Matched: true, Recovery: recovery, Root: input.Root, FingerprintInput: fp, Window: 10 * time.Minute, ReopenWindow: 30 * time.Minute, Severity: "critical", Evidence: []Evidence{{Kind: "signal", Summary: boundedSummary(input.Signal.Message, 512), Attributes: map[string]string{"reason": input.Signal.Reason, "container": container, "source": string(input.Signal.Source)}}}, Explain: Explain{MatchedConditions: conditions, Reason: "container restart failures are grouped by stable root resource and container"}}, nil
}

func isCrashLoopSignal(item signal.Signal) bool {
	reason := strings.ToLower(item.Reason)
	message := strings.ToLower(item.Message)
	if item.Source == signal.SourceKubernetes && item.Type == signal.TypeNormal && reason == "started" {
		return true
	}
	if strings.Contains(reason, "imagepull") || reason == "errimagepull" || strings.Contains(message, "pulling image") || strings.Contains(message, "pull image") || strings.Contains(message, "image pull") {
		return false
	}
	if item.Source == signal.SourceKubernetes {
		return reason == "backoff" && (strings.Contains(message, "restart") || strings.Contains(message, "failed container") || message == "")
	}
	return strings.Contains(reason, "crashloop") || strings.Contains(reason, "containerrestart") || strings.Contains(reason, "containerrestarting")
}
