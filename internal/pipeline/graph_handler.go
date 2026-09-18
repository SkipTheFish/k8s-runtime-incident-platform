package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"observability-platform/internal/correlation"
	"observability-platform/internal/incident"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

type SignalStore interface {
	Persist(context.Context, signal.Signal) error
	MarkMatched(context.Context, string, resourcegraph.ResourceKey) error
	MarkUnmatched(context.Context, string, string) error
	RecordDecision(context.Context, signal.Signal, correlation.MatchResult) error
}

type CorrelationEvaluator interface {
	Evaluate(context.Context, correlation.EvaluationInput) (correlation.MatchResult, error)
}
type IncidentProcessor interface {
	Process(context.Context, signal.Signal, correlation.MatchResult) (incident.ProcessResult, error)
}

type ResolutionError struct {
	Cause     error
	retryable bool
}

func (e *ResolutionError) Error() string   { return e.Cause.Error() }
func (e *ResolutionError) Unwrap() error   { return e.Cause }
func (e *ResolutionError) Retryable() bool { return e.retryable }

type GraphHandler struct {
	store     SignalStore
	mu        sync.RWMutex
	resolver  *resourcegraph.Resolver
	evaluator CorrelationEvaluator
	metrics   *CorrelationMetrics
	incidents IncidentProcessor
}

func NewGraphHandler(store SignalStore) *GraphHandler { return &GraphHandler{store: store} }

func (h *GraphHandler) SetResolver(resolver *resourcegraph.Resolver) {
	h.mu.Lock()
	h.resolver = resolver
	h.mu.Unlock()
}

func (h *GraphHandler) SetEvaluator(evaluator CorrelationEvaluator) {
	h.mu.Lock()
	h.evaluator = evaluator
	h.mu.Unlock()
}
func (h *GraphHandler) SetCorrelationMetrics(metrics *CorrelationMetrics) { h.metrics = metrics }
func (h *GraphHandler) SetIncidentProcessor(processor IncidentProcessor)  { h.incidents = processor }

func (h *GraphHandler) Handle(ctx context.Context, item signal.Signal) error {
	if err := h.store.Persist(ctx, item); err != nil && !errors.Is(err, ErrDuplicate) {
		return err
	}
	if item.Resource.UID == "" {
		err := &ResolutionError{Cause: errors.New("signal resource UID is empty"), retryable: false}
		if updateErr := h.store.MarkUnmatched(ctx, item.ID, err.Error()); updateErr != nil {
			return updateErr
		}
		return err
	}
	h.mu.RLock()
	resolver := h.resolver
	h.mu.RUnlock()
	if resolver == nil {
		return &ResolutionError{Cause: errors.New("resource informer indexes are not ready"), retryable: true}
	}
	result, err := resolver.Resolve(resourcegraph.ResourceKey{ClusterID: item.ClusterID, APIVersion: item.Resource.APIVersion, Kind: item.Resource.Kind, Namespace: item.Resource.Namespace, Name: item.Resource.Name, UID: item.Resource.UID})
	if err != nil {
		retryable := errors.Is(err, resourcegraph.ErrResourceNotFound) || errors.Is(err, resourcegraph.ErrOwnerNotFound) || errors.Is(err, resourcegraph.ErrOrphanPod)
		classified := &ResolutionError{Cause: fmt.Errorf("resolve signal root: %w", err), retryable: retryable}
		if !retryable {
			if updateErr := h.store.MarkUnmatched(ctx, item.ID, classified.Error()); updateErr != nil {
				return updateErr
			}
		}
		return classified
	}
	h.mu.RLock()
	evaluator := h.evaluator
	h.mu.RUnlock()
	if evaluator == nil {
		return h.store.MarkMatched(ctx, item.ID, result.Root.Key)
	}
	started := time.Now()
	match, evaluateErr := evaluator.Evaluate(ctx, correlation.EvaluationInput{Signal: item, Root: result.Root.Key})
	if h.metrics != nil {
		h.metrics.duration.Observe(time.Since(started).Seconds())
	}
	if errors.Is(evaluateErr, correlation.ErrNoMatch) {
		if err := h.store.MarkUnmatched(ctx, item.ID, correlation.ErrNoMatch.Error()); err != nil {
			return err
		}
		if h.metrics != nil {
			h.metrics.unmatched.Inc()
		}
		return h.store.RecordDecision(ctx, item, match)
	}
	if evaluateErr != nil {
		var conflict *correlation.ConflictError
		if errors.As(evaluateErr, &conflict) {
			if h.metrics != nil {
				h.metrics.conflicts.Inc()
			}
			slog.Error("correlation policy conflict", "signalID", item.ID, "priority", conflict.Priority, "rules", conflict.Rules)
			_ = h.store.MarkUnmatched(ctx, item.ID, evaluateErr.Error())
			_ = h.store.RecordDecision(ctx, item, correlation.MatchResult{Matched: false, Root: result.Root.Key, Explain: correlation.Explain{Reason: evaluateErr.Error()}})
			return &ResolutionError{Cause: evaluateErr, retryable: false}
		}
		return evaluateErr
	}
	if h.incidents != nil {
		_, err := h.incidents.Process(ctx, item, match)
		return err
	}
	if err := h.store.MarkMatched(ctx, item.ID, result.Root.Key); err != nil {
		return err
	}
	return h.store.RecordDecision(ctx, item, match)
}
