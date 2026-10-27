package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"observability-platform/internal/correlation"
	"observability-platform/internal/incident"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	ctx, span := otel.Tracer("observability-platform/pipeline").Start(ctx, "signal.process")
	defer span.End()
	span.SetAttributes(attribute.String("signal.id", item.ID), attribute.String("signal.source", string(item.Source)), attribute.String("signal.type", string(item.Type)), attribute.String("resource.uid", item.Resource.UID))
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
	resourceKey := resourcegraph.ResourceKey{ClusterID: item.ClusterID, APIVersion: item.Resource.APIVersion, Kind: item.Resource.Kind, Namespace: item.Resource.Namespace, Name: item.Resource.Name, UID: item.Resource.UID}
	var result resourcegraph.RootResolutionResult
	var err error
	if isFailedSchedulingPod(item) {
		// Keep both FailedScheduling and Scheduled on the Pod root. Once a Pod is
		// scheduled, the generic resolver follows spec.nodeName to the Node. That
		// changes the correlation fingerprint and prevents the Scheduled event from
		// resolving the incident created while the same Pod was pending.
		result.Root = resourcegraph.ResourceNode{Key: resourceKey}
	} else {
		result, err = resolver.Resolve(resourceKey)
	}
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

func isFailedSchedulingPod(item signal.Signal) bool {
	reason := strings.TrimSpace(item.Reason)
	return (strings.EqualFold(reason, "FailedScheduling") || strings.EqualFold(reason, "Scheduled")) && strings.EqualFold(strings.TrimSpace(item.Resource.Kind), "Pod")
}
