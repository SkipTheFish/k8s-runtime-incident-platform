package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

type SignalStore interface {
	Persist(context.Context, signal.Signal) error
	MarkMatched(context.Context, string, resourcegraph.ResourceKey) error
	MarkUnmatched(context.Context, string, string) error
}

type ResolutionError struct {
	Cause     error
	retryable bool
}

func (e *ResolutionError) Error() string   { return e.Cause.Error() }
func (e *ResolutionError) Unwrap() error   { return e.Cause }
func (e *ResolutionError) Retryable() bool { return e.retryable }

type GraphHandler struct {
	store    SignalStore
	mu       sync.RWMutex
	resolver *resourcegraph.Resolver
}

func NewGraphHandler(store SignalStore) *GraphHandler { return &GraphHandler{store: store} }

func (h *GraphHandler) SetResolver(resolver *resourcegraph.Resolver) {
	h.mu.Lock()
	h.resolver = resolver
	h.mu.Unlock()
}

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
	return h.store.MarkMatched(ctx, item.ID, result.Root.Key)
}
