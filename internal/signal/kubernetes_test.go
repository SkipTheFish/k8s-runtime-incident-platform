package signal

import (
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestFromKubernetesEvent(t *testing.T) {
	for _, reason := range []string{"ImagePullBackOff", "ErrImagePull", "Failed", "BackOff", "FailedScheduling"} {
		t.Run(reason, func(t *testing.T) {
			sourceTime := time.Date(2026, 9, 1, 1, 2, 3, 0, time.UTC)
			event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "event-1", Namespace: "default", UID: types.UID("event-uid")}, Type: corev1.EventTypeWarning, Reason: reason, Message: "failure", EventTime: metav1.MicroTime{Time: sourceTime}, InvolvedObject: corev1.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod-a", UID: types.UID("pod-uid")}}
			got, err := FromKubernetesEvent("cluster-a", event, sourceTime.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			if got.Resource.UID != "pod-uid" || got.OccurredAt != sourceTime || got.ID == "" {
				t.Fatalf("incomplete signal: %+v", got)
			}
			replayed, _ := FromKubernetesEvent("cluster-a", event, sourceTime.Add(2*time.Minute))
			if replayed.ID != got.ID {
				t.Fatal("replay changed stable ID")
			}
		})
	}
}

func TestFromKubernetesEventExtractsContainer(t *testing.T) {
	now := time.Now()
	event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{UID: "event"}, Type: corev1.EventTypeWarning, Reason: "ImagePullBackOff", EventTime: metav1.MicroTime{Time: now}, InvolvedObject: corev1.ObjectReference{UID: "pod", Kind: "Pod", Name: "pod", Namespace: "ns", FieldPath: "spec.containers{api}"}}
	got, err := FromKubernetesEvent("c", event, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Labels["container"] != "api" {
		t.Fatalf("labels=%v", got.Labels)
	}
}

func TestKubernetesEventFiltering(t *testing.T) {
	event := &corev1.Event{Type: corev1.EventTypeNormal, Reason: "Pulled", LastTimestamp: metav1.Now()}
	_, err := FromKubernetesEvent("cluster", event, time.Now())
	if !errors.Is(err, ErrFiltered) {
		t.Fatalf("expected filtered error, got %v", err)
	}
}

func TestKubernetesEventTimestampPrecedenceAndFallbacks(t *testing.T) {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*corev1.Event)
		want   time.Time
	}{
		{"event time", func(event *corev1.Event) { event.EventTime = metav1.MicroTime{Time: base.Add(5 * time.Minute)} }, base.Add(5 * time.Minute)},
		{"series", func(event *corev1.Event) {
			event.Series = &corev1.EventSeries{LastObservedTime: metav1.MicroTime{Time: base.Add(4 * time.Minute)}}
		}, base.Add(4 * time.Minute)},
		{"last timestamp", func(event *corev1.Event) { event.LastTimestamp = metav1.Time{Time: base.Add(3 * time.Minute)} }, base.Add(3 * time.Minute)},
		{"first timestamp", func(event *corev1.Event) { event.FirstTimestamp = metav1.Time{Time: base.Add(2 * time.Minute)} }, base.Add(2 * time.Minute)},
		{"creation timestamp", func(event *corev1.Event) { event.CreationTimestamp = metav1.Time{Time: base.Add(time.Minute)} }, base.Add(time.Minute)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := &corev1.Event{Type: corev1.EventTypeWarning, Reason: "BackOff", ObjectMeta: metav1.ObjectMeta{Name: "event", Namespace: "ns"}, InvolvedObject: corev1.ObjectReference{UID: "pod"}}
			test.mutate(event)
			got, err := FromKubernetesEvent("cluster", event, base)
			if err != nil {
				t.Fatal(err)
			}
			if !got.OccurredAt.Equal(test.want) || got.SourceEventKey != "ns/event" {
				t.Fatalf("occurredAt=%s sourceKey=%q", got.OccurredAt, got.SourceEventKey)
			}
		})
	}
}

func TestKubernetesEventRejectsMissingTimestampAndMapsOwners(t *testing.T) {
	event := &corev1.Event{Type: corev1.EventTypeWarning, Reason: "BackOff", InvolvedObject: corev1.ObjectReference{UID: "pod"}}
	if _, err := FromKubernetesEvent("cluster", event, time.Now()); err == nil {
		t.Fatal("event without source timestamp was accepted")
	}
	event.EventTime = metav1.MicroTime{Time: time.Now()}
	event.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "api", UID: "deployment"}}
	got, err := FromKubernetesEvent("cluster", event, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].UID != "deployment" {
		t.Fatalf("owners=%#v", got.OwnerReferences)
	}
}
