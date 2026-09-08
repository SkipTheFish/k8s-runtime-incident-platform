package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

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
