package correlation

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

var (
	ErrSimulationLimit          = errors.New("simulation input limit exceeded")
	ErrSimulationPolicyNotFound = errors.New("correlation policy not found")
)

type SimulationInput struct {
	Signal signal.Signal             `json:"signal"`
	Root   resourcegraph.ResourceKey `json:"root"`
}
type SimulationRequest struct {
	Generation          int64             `json:"generation"`
	HistoricalSignalIDs []string          `json:"historicalSignalIds,omitempty"`
	Signals             []SimulationInput `json:"signals,omitempty"`
}
type SimulatedIncident struct {
	Fingerprint string                    `json:"fingerprint"`
	RuleName    string                    `json:"ruleName"`
	RuleVersion string                    `json:"ruleVersion"`
	Root        resourcegraph.ResourceKey `json:"root"`
	SignalIDs   []string                  `json:"signalIds"`
	Recoveries  int                       `json:"recoveries"`
	Explain     Explain                   `json:"explain"`
}
type UnmatchedSimulation struct {
	SignalID string `json:"signalId"`
	Reason   string `json:"reason"`
}
type SimulationResult struct {
	InputCount int                   `json:"inputCount"`
	Incidents  []SimulatedIncident   `json:"incidents"`
	Unmatched  []UnmatchedSimulation `json:"unmatched"`
}
type HistoricalSignalReader interface {
	LoadSimulationInputs(context.Context, []string, int) ([]SimulationInput, error)
}
type Simulator struct {
	registry  *Registry
	reader    HistoricalSignalReader
	maxInputs int
}

func NewSimulator(registry *Registry, reader HistoricalSignalReader, maxInputs int) *Simulator {
	if maxInputs < 1 {
		maxInputs = 1000
	}
	return &Simulator{registry: registry, reader: reader, maxInputs: maxInputs}
}

func (s *Simulator) Simulate(ctx context.Context, policyName string, request SimulationRequest) (SimulationResult, error) {
	if len(request.HistoricalSignalIDs)+len(request.Signals) > s.maxInputs {
		return SimulationResult{}, ErrSimulationLimit
	}
	rule, ok := s.registry.Find(policyName)
	if !ok {
		return SimulationResult{}, ErrSimulationPolicyNotFound
	}
	inputs := append([]SimulationInput(nil), request.Signals...)
	if len(request.HistoricalSignalIDs) > 0 {
		if s.reader == nil {
			return SimulationResult{}, errors.New("historical signal source is unavailable")
		}
		historical, err := s.reader.LoadSimulationInputs(ctx, request.HistoricalSignalIDs, s.maxInputs-len(inputs))
		if err != nil {
			return SimulationResult{}, fmt.Errorf("load historical signals: %w", err)
		}
		inputs = append(inputs, historical...)
	}
	if len(inputs) > s.maxInputs {
		return SimulationResult{}, ErrSimulationLimit
	}
	sort.SliceStable(inputs, func(i, j int) bool { return inputs[i].Signal.ID < inputs[j].Signal.ID })
	engine := NewRegistry(rule)
	result := SimulationResult{InputCount: len(inputs), Incidents: []SimulatedIncident{}, Unmatched: []UnmatchedSimulation{}}
	byFingerprint := map[string]int{}
	for _, input := range inputs {
		if input.Signal.ID == "" {
			input.Signal = signal.Normalize(input.Signal)
		}
		match, err := engine.Evaluate(ctx, EvaluationInput{Signal: input.Signal, Root: input.Root, PolicyGeneration: request.Generation})
		if errors.Is(err, ErrNoMatch) {
			result.Unmatched = append(result.Unmatched, UnmatchedSimulation{SignalID: input.Signal.ID, Reason: match.Explain.Reason})
			continue
		}
		if err != nil {
			return SimulationResult{}, err
		}
		index, exists := byFingerprint[match.Fingerprint]
		if !exists {
			index = len(result.Incidents)
			byFingerprint[match.Fingerprint] = index
			result.Incidents = append(result.Incidents, SimulatedIncident{Fingerprint: match.Fingerprint, RuleName: match.RuleName, RuleVersion: match.RuleVersion, Root: match.Root, SignalIDs: []string{}, Explain: match.Explain})
		}
		result.Incidents[index].SignalIDs = append(result.Incidents[index].SignalIDs, input.Signal.ID)
		if match.Recovery {
			result.Incidents[index].Recoveries++
		}
	}
	return result, nil
}
