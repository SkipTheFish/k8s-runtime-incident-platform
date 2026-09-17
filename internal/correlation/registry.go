package correlation

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var ErrNoMatch = errors.New("no correlation rule matched")

type ConflictError struct {
	Priority int
	Rules    []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("multiple correlation rules matched at priority %d: %s", e.Priority, strings.Join(e.Rules, ", "))
}

type Registry struct {
	mu    sync.RWMutex
	rules []CorrelationRule
}

func NewRegistry(rules ...CorrelationRule) *Registry {
	r := &Registry{}
	r.Replace(rules)
	return r
}

func (r *Registry) Replace(rules []CorrelationRule) {
	copyOfRules := append([]CorrelationRule(nil), rules...)
	sort.SliceStable(copyOfRules, func(i, j int) bool {
		if copyOfRules[i].Priority() == copyOfRules[j].Priority() {
			return copyOfRules[i].Name() < copyOfRules[j].Name()
		}
		return copyOfRules[i].Priority() > copyOfRules[j].Priority()
	})
	r.mu.Lock()
	r.rules = copyOfRules
	r.mu.Unlock()
}

func (r *Registry) Snapshot() []CorrelationRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]CorrelationRule(nil), r.rules...)
}

func (r *Registry) Find(name string) (CorrelationRule, bool) {
	for _, rule := range r.Snapshot() {
		if rule.Name() == name || strings.HasSuffix(rule.Name(), "/"+name) {
			return rule, true
		}
	}
	return nil, false
}

func (r *Registry) Evaluate(ctx context.Context, input EvaluationInput) (MatchResult, error) {
	rules := r.Snapshot()
	for index := 0; index < len(rules); {
		priority := rules[index].Priority()
		var matches []MatchResult
		for index < len(rules) && rules[index].Priority() == priority {
			rule := rules[index]
			index++
			if !rule.Enabled() {
				continue
			}
			result, err := rule.Match(ctx, input)
			if err != nil {
				return MatchResult{}, fmt.Errorf("evaluate correlation rule %s: %w", rule.Name(), err)
			}
			if !result.Matched {
				continue
			}
			result.RuleName = rule.Name()
			result.RuleVersion = rule.Version()
			result.PolicyGeneration = input.PolicyGeneration
			if result.Fingerprint == "" {
				result.FingerprintInput.Rule = rule.Name()
				result.FingerprintInput.RuleVersion = rule.Version()
				result.Fingerprint = result.FingerprintInput.Fingerprint()
			}
			matches = append(matches, result)
		}
		if len(matches) > 1 {
			names := make([]string, len(matches))
			for i := range matches {
				names[i] = matches[i].RuleName
			}
			return MatchResult{}, &ConflictError{Priority: priority, Rules: names}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return MatchResult{Matched: false, PolicyGeneration: input.PolicyGeneration, Root: input.Root, Explain: Explain{Reason: ErrNoMatch.Error()}}, ErrNoMatch
}
