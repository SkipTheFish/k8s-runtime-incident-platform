package correlation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

type EvaluationInput struct {
	Signal           signal.Signal             `json:"signal"`
	Root             resourcegraph.ResourceKey `json:"root"`
	PolicyGeneration int64                     `json:"policyGeneration"`
}

type FingerprintInput struct {
	Rule            string            `json:"rule"`
	RuleVersion     string            `json:"ruleVersion"`
	ClusterID       string            `json:"clusterId"`
	RootResourceUID string            `json:"rootResourceUid"`
	Dimensions      map[string]string `json:"dimensions,omitempty"`
}

func (f FingerprintInput) Fingerprint() string {
	payload, _ := json.Marshal(f)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

type Evidence struct {
	Kind       string            `json:"kind"`
	Summary    string            `json:"summary"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Explain struct {
	MatchedConditions []string `json:"matchedConditions,omitempty"`
	Reason            string   `json:"reason"`
}

type MatchResult struct {
	Matched          bool                      `json:"matched"`
	Recovery         bool                      `json:"recovery"`
	RuleName         string                    `json:"ruleName,omitempty"`
	RuleVersion      string                    `json:"ruleVersion,omitempty"`
	PolicyGeneration int64                     `json:"policyGeneration"`
	Fingerprint      string                    `json:"fingerprint,omitempty"`
	FingerprintInput FingerprintInput          `json:"fingerprintInput,omitempty"`
	Root             resourcegraph.ResourceKey `json:"root,omitempty"`
	Window           time.Duration             `json:"window,omitempty"`
	ReopenWindow     time.Duration             `json:"reopenWindow,omitempty"`
	Severity         string                    `json:"severity,omitempty"`
	Evidence         []Evidence                `json:"evidence,omitempty"`
	Explain          Explain                   `json:"explain"`
}

type CorrelationRule interface {
	Name() string
	Version() string
	Priority() int
	Enabled() bool
	Match(context.Context, EvaluationInput) (MatchResult, error)
}
