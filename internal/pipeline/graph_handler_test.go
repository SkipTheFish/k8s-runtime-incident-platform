package pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"observability-platform/internal/correlation"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type memorySignalStore struct {
	mu        sync.Mutex
	persist   int
	matched   chan resourcegraph.ResourceKey
	unmatched int
	decisions []correlation.MatchResult
}

func (s *memorySignalStore) RecordDecision(_ context.Context, _ signal.Signal, result correlation.MatchResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions = append(s.decisions, result)
	return nil
}

type evaluatorFunc func(context.Context, correlation.EvaluationInput) (correlation.MatchResult, error)

func (f evaluatorFunc) Evaluate(ctx context.Context, input correlation.EvaluationInput) (correlation.MatchResult, error) {
	return f(ctx, input)
}

func (s *memorySignalStore) Persist(context.Context, signal.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persist++
	if s.persist > 1 {
		return ErrDuplicate
	}
	return nil
}
func (s *memorySignalStore) MarkMatched(_ context.Context, _ string, root resourcegraph.ResourceKey) error {
	s.matched <- root
	return nil
}
func (s *memorySignalStore) MarkUnmatched(context.Context, string, string) error {
	s.mu.Lock()
	s.unmatched++
	s.mu.Unlock()
	return nil
}

func TestGraphHandlerRetriesWhenOwnerArrivesAfterSignal(t *testing.T) {
	podIndexer := newGraphIndexer()
	jobIndexer := newGraphIndexer()
	controller := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns", UID: types.UID("pod-1"), OwnerReferences: []metav1.OwnerReference{{UID: types.UID("job-1"), Controller: &controller}}}}
	if err := podIndexer.Add(pod); err != nil {
		t.Fatal(err)
	}
	resolver := resourcegraph.NewResolver([]resourcegraph.IndexedResource{
		{Indexer: podIndexer, GVK: schema.FromAPIVersionAndKind("v1", "Pod")},
		{Indexer: jobIndexer, GVK: schema.FromAPIVersionAndKind("batch/v1", "Job")},
	}, 8)
	store := &memorySignalStore{matched: make(chan resourcegraph.ResourceKey, 1)}
	handler := NewGraphHandler(store)
	handler.SetResolver(resolver)
	firstAttempt := make(chan struct{}, 1)
	wrapped := func(ctx context.Context, item signal.Signal) error {
		err := handler.Handle(ctx, item)
		if err != nil {
			select {
			case firstAttempt <- struct{}{}:
			default:
			}
		}
		return err
	}
	queue := NewQueue(Config{Workers: 1, MaxRetries: 5, BaseDelay: 20 * time.Millisecond, MaxDelay: 20 * time.Millisecond}, wrapped, nil)
	item := signal.Signal{ID: "signal-1", ClusterID: "c", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "pod", UID: "pod-1"}}
	if err := queue.Enqueue(item); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- queue.Run(ctx) }()
	select {
	case <-firstAttempt:
	case <-time.After(time.Second):
		t.Fatal("first resolution was not attempted")
	}
	if err := jobIndexer.Add(&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "ns", UID: types.UID("job-1")}}); err != nil {
		t.Fatal(err)
	}
	select {
	case root := <-store.matched:
		if root.Kind != "Job" || root.UID != "job-1" {
			t.Fatalf("root=%#v", root)
		}
	case <-time.After(time.Second):
		t.Fatal("signal was not matched after owner arrived")
	}
	cancel()
	<-done
	store.mu.Lock()
	persists := store.persist
	store.mu.Unlock()
	if persists < 2 {
		t.Fatalf("persist attempts=%d", persists)
	}
}

func TestGraphHandlerKeepsUIDLessSignalAsUnmatched(t *testing.T) {
	store := &memorySignalStore{matched: make(chan resourcegraph.ResourceKey, 1)}
	err := NewGraphHandler(store).Handle(context.Background(), signal.Signal{ID: "signal-1"})
	classified, ok := err.(*ResolutionError)
	if !ok || classified.Retryable() {
		t.Fatalf("error=%v", err)
	}
	store.mu.Lock()
	unmatched := store.unmatched
	store.mu.Unlock()
	if unmatched != 1 {
		t.Fatalf("unmatched=%d", unmatched)
	}
}

func TestGraphHandlerUsesOrphanPodAsRootOnlyForFailedScheduling(t *testing.T) {
	podIndexer := newGraphIndexer()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pending", Namespace: "ns", UID: types.UID("pod-1")}, Spec: corev1.PodSpec{NodeName: "worker-1"}}
	if err := podIndexer.Add(pod); err != nil {
		t.Fatal(err)
	}
	nodeIndexer := newGraphIndexer()
	if err := nodeIndexer.Add(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1", UID: types.UID("node-1")}}); err != nil {
		t.Fatal(err)
	}
	resolver := resourcegraph.NewResolver([]resourcegraph.IndexedResource{
		{Indexer: podIndexer, GVK: schema.FromAPIVersionAndKind("v1", "Pod")},
		{Indexer: nodeIndexer, GVK: schema.FromAPIVersionAndKind("v1", "Node")},
	}, 8)

	t.Run("FailedScheduling reaches evaluator with Pod root", func(t *testing.T) {
		store := &memorySignalStore{matched: make(chan resourcegraph.ResourceKey, 1)}
		handler := NewGraphHandler(store)
		handler.SetResolver(resolver)
		evaluated := false
		handler.SetEvaluator(evaluatorFunc(func(_ context.Context, input correlation.EvaluationInput) (correlation.MatchResult, error) {
			evaluated = true
			if input.Root.Kind != "Pod" || input.Root.UID != "pod-1" || input.Root.Name != "pending" {
				t.Fatalf("root=%#v", input.Root)
			}
			return correlation.MatchResult{Matched: true, Root: input.Root, RuleName: "failed-scheduling", RuleVersion: "v1", Fingerprint: "fp"}, nil
		}))
		item := signal.Signal{ID: "failed-scheduling", ClusterID: "c", Reason: "FailedScheduling", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "pending", UID: "pod-1"}}
		if err := handler.Handle(context.Background(), item); err != nil {
			t.Fatal(err)
		}
		if !evaluated {
			t.Fatal("evaluator was not called")
		}
		select {
		case root := <-store.matched:
			if root.Kind != "Pod" || root.UID != "pod-1" {
				t.Fatalf("matched root=%#v", root)
			}
		default:
			t.Fatal("signal was not marked matched")
		}
	})

	t.Run("Scheduled recovery keeps same Pod root after node assignment", func(t *testing.T) {
		store := &memorySignalStore{matched: make(chan resourcegraph.ResourceKey, 1)}
		handler := NewGraphHandler(store)
		handler.SetResolver(resolver)
		handler.SetEvaluator(evaluatorFunc(func(_ context.Context, input correlation.EvaluationInput) (correlation.MatchResult, error) {
			if input.Root.Kind != "Pod" || input.Root.UID != "pod-1" {
				t.Fatalf("root=%#v", input.Root)
			}
			return correlation.MatchResult{Matched: true, Recovery: true, Root: input.Root, RuleName: "failed-scheduling", RuleVersion: "v1", Fingerprint: "fp"}, nil
		}))
		item := signal.Signal{ID: "scheduled", ClusterID: "c", Type: signal.TypeNormal, Reason: "Scheduled", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "pending", UID: "pod-1"}}
		if err := handler.Handle(context.Background(), item); err != nil {
			t.Fatal(err)
		}
		select {
		case root := <-store.matched:
			if root.UID != "pod-1" {
				t.Fatalf("matched root=%#v", root)
			}
		default:
			t.Fatal("recovery signal was not marked matched")
		}
	})

	t.Run("other orphan Pod remains retryable", func(t *testing.T) {
		orphanIndexer := newGraphIndexer()
		if err := orphanIndexer.Add(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "orphan", Namespace: "ns", UID: types.UID("pod-2")}}); err != nil {
			t.Fatal(err)
		}
		orphanResolver := resourcegraph.NewResolver([]resourcegraph.IndexedResource{{Indexer: orphanIndexer, GVK: schema.FromAPIVersionAndKind("v1", "Pod")}}, 8)
		store := &memorySignalStore{matched: make(chan resourcegraph.ResourceKey, 1)}
		handler := NewGraphHandler(store)
		handler.SetResolver(orphanResolver)
		handler.SetEvaluator(evaluatorFunc(func(context.Context, correlation.EvaluationInput) (correlation.MatchResult, error) {
			t.Fatal("evaluator must not be called for a non-scheduling orphan Pod")
			return correlation.MatchResult{}, nil
		}))
		item := signal.Signal{ID: "backoff", ClusterID: "c", Reason: "BackOff", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "orphan", UID: "pod-2"}}
		err := handler.Handle(context.Background(), item)
		var classified *ResolutionError
		if !errors.As(err, &classified) || !classified.Retryable() || !errors.Is(err, resourcegraph.ErrOrphanPod) {
			t.Fatalf("error=%v", err)
		}
	})
}

func newGraphIndexer() cache.Indexer {
	return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		resourcegraph.UIDIndex: func(obj any) ([]string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return nil, err
			}
			return []string{string(accessor.GetUID())}, nil
		},
		"resourceGraphName": func(obj any) ([]string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return nil, err
			}
			return []string{accessor.GetNamespace() + "\x00" + accessor.GetName()}, nil
		},
	})
}

func TestGraphHandlerPersistsMatchedAndUnmatchedDecisions(t *testing.T) {
	nodeIndexer := newGraphIndexer()
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: types.UID("node-1")}}
	if err := nodeIndexer.Add(node); err != nil {
		t.Fatal(err)
	}
	resolver := resourcegraph.NewResolver([]resourcegraph.IndexedResource{{Indexer: nodeIndexer, GVK: schema.FromAPIVersionAndKind("v1", "Node")}}, 8)
	tests := []struct {
		name                       string
		evaluator                  evaluatorFunc
		wantMatched, wantUnmatched bool
	}{
		{"matched", func(_ context.Context, input correlation.EvaluationInput) (correlation.MatchResult, error) {
			return correlation.MatchResult{Matched: true, Root: input.Root, RuleName: "rule", RuleVersion: "v1", Fingerprint: "fp"}, nil
		}, true, false},
		{"unmatched", func(_ context.Context, input correlation.EvaluationInput) (correlation.MatchResult, error) {
			return correlation.MatchResult{Root: input.Root, Explain: correlation.Explain{Reason: correlation.ErrNoMatch.Error()}}, correlation.ErrNoMatch
		}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &memorySignalStore{matched: make(chan resourcegraph.ResourceKey, 1)}
			handler := NewGraphHandler(store)
			handler.SetResolver(resolver)
			handler.SetEvaluator(tt.evaluator)
			item := signal.Signal{ID: "s", ClusterID: "c", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Node", Name: "node", UID: "node-1"}}
			if err := handler.Handle(context.Background(), item); err != nil {
				t.Fatal(err)
			}
			store.mu.Lock()
			decisions := len(store.decisions)
			unmatched := store.unmatched
			store.mu.Unlock()
			if decisions != 1 || (unmatched > 0) != tt.wantUnmatched {
				t.Fatalf("decisions=%d unmatched=%d", decisions, unmatched)
			}
			select {
			case <-store.matched:
				if !tt.wantMatched {
					t.Fatal("unexpected matched")
				}
			default:
				if tt.wantMatched {
					t.Fatal("not marked matched")
				}
			}
		})
	}
}
