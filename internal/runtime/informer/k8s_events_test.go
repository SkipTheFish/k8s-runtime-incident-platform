package informer

import (
	"testing"
	"time"

	"observability-platform/internal/signal"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func TestEventFromObject(t *testing.T) {
	event := &corev1.Event{}
	for _, obj := range []any{event, cache.DeletedFinalStateUnknown{Key: "event", Obj: event}} {
		got, ok := eventFromObject(obj)
		if !ok || got != event {
			t.Fatalf("failed to extract event from %T", obj)
		}
	}
	if _, ok := eventFromObject("wrong"); ok {
		t.Fatal("unexpected extraction success")
	}
}

func TestHandleAddUpdateAndTombstone(t *testing.T) {
	event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "e", Namespace: "default", UID: types.UID("event")}, Type: corev1.EventTypeWarning, Reason: "BackOff", LastTimestamp: metav1.NewTime(time.Now()), InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "default", Name: "pod", UID: types.UID("pod")}}
	var received int
	informer := &K8sEventInformer{clusterID: "cluster", enqueue: func(_ signal.Signal) error { received++; return nil }}
	informer.handle(event)
	informer.handle(event.DeepCopy())
	informer.handle(cache.DeletedFinalStateUnknown{Key: "e", Obj: event})
	if received != 3 {
		t.Fatalf("received=%d, want 3", received)
	}
}
