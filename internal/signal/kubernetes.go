package signal

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var ErrFiltered = errors.New("event filtered")

var acceptedWarningReasons = map[string]struct{}{
	"ImagePullBackOff": {}, "ErrImagePull": {}, "Failed": {}, "BackOff": {}, "FailedScheduling": {},
}

var acceptedNormalReasons = map[string]struct{}{
	"Pulled": {}, "Started": {}, "Scheduled": {},
}

var containerFieldPath = regexp.MustCompile(`(?:initContainers|containers)\{([^}]+)\}`)

func FromKubernetesEvent(clusterID string, event *corev1.Event, observedAt time.Time) (Signal, error) {
	signalType := TypeWarning
	switch event.Type {
	case corev1.EventTypeWarning:
		if _, ok := acceptedWarningReasons[event.Reason]; !ok {
			return Signal{}, fmt.Errorf("%w: warning reason %s", ErrFiltered, event.Reason)
		}
	case corev1.EventTypeNormal:
		if _, ok := acceptedNormalReasons[event.Reason]; !ok {
			return Signal{}, fmt.Errorf("%w: normal reason %s", ErrFiltered, event.Reason)
		}
		signalType = TypeNormal
	default:
		return Signal{}, fmt.Errorf("%w: type %s", ErrFiltered, event.Type)
	}
	occurredAt := kubernetesEventTime(event)
	if occurredAt.IsZero() {
		return Signal{}, fmt.Errorf("event has no source timestamp")
	}
	owners := make([]ObjectReference, 0, len(event.OwnerReferences))
	for _, owner := range event.OwnerReferences {
		owners = append(owners, ObjectReference{APIVersion: owner.APIVersion, Kind: owner.Kind, Name: owner.Name, UID: string(owner.UID)})
	}
	sourceKey := string(event.UID)
	if sourceKey == "" {
		sourceKey = event.Namespace + "/" + event.Name
	}
	s := Signal{
		ClusterID: clusterID, Source: SourceKubernetes, Type: signalType,
		SourceEventKey: sourceKey, Reason: event.Reason, Namespace: event.InvolvedObject.Namespace,
		Resource:        ObjectReference{APIVersion: event.InvolvedObject.APIVersion, Kind: event.InvolvedObject.Kind, Namespace: event.InvolvedObject.Namespace, Name: event.InvolvedObject.Name, UID: string(event.InvolvedObject.UID)},
		OwnerReferences: owners, Message: event.Message, OccurredAt: occurredAt, ObservedAt: observedAt,
	}
	if match := containerFieldPath.FindStringSubmatch(event.InvolvedObject.FieldPath); len(match) == 2 {
		s.Labels = map[string]string{"container": match[1]}
	}
	return Normalize(s), nil
}

func kubernetesEventTime(event *corev1.Event) time.Time {
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if event.Series != nil && !event.Series.LastObservedTime.IsZero() {
		return event.Series.LastObservedTime.Time
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	return event.CreationTimestamp.Time
}
