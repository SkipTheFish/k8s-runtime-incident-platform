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
	for _, reason := range []string{"ImagePullBackOff", "ErrImagePull", "BackOff", "FailedScheduling"} {
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

func TestKubernetesEventFiltering(t *testing.T) {
	event := &corev1.Event{Type: corev1.EventTypeNormal, Reason: "Pulled", LastTimestamp: metav1.Now()}
	_, err := FromKubernetesEvent("cluster", event, time.Now())
	if !errors.Is(err, ErrFiltered) {
		t.Fatalf("expected filtered error, got %v", err)
	}
}
