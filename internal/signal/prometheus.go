package signal

import (
	"fmt"
	"time"
)

type PrometheusAlert struct {
	Fingerprint string
	Status      string
	Labels      map[string]string
	Annotations map[string]string
	StartsAt    time.Time
	EndsAt      time.Time
}

func FromPrometheusAlert(clusterID string, alert PrometheusAlert, observedAt time.Time) (Signal, error) {
	for _, key := range []string{"alertname", "namespace", "resource_kind", "resource_name", "resource_uid"} {
		if alert.Labels[key] == "" {
			return Signal{}, fmt.Errorf("prometheus alert missing label %s", key)
		}
	}
	if alert.Fingerprint == "" {
		return Signal{}, fmt.Errorf("prometheus alert missing fingerprint")
	}
	var signalType Type
	var occurredAt time.Time
	switch alert.Status {
	case "firing":
		signalType, occurredAt = TypeAlertFiring, alert.StartsAt
	case "resolved":
		signalType, occurredAt = TypeAlertResolved, alert.EndsAt
	default:
		return Signal{}, fmt.Errorf("unsupported prometheus alert status %q", alert.Status)
	}
	if occurredAt.IsZero() {
		return Signal{}, fmt.Errorf("prometheus alert missing source timestamp")
	}
	s := Signal{
		ClusterID: clusterID, Source: SourcePrometheus, Type: signalType,
		SourceEventKey: alert.Fingerprint, Reason: alert.Labels["alertname"], Namespace: alert.Labels["namespace"],
		Resource: ObjectReference{APIVersion: alert.Labels["resource_api_version"], Kind: alert.Labels["resource_kind"], Namespace: alert.Labels["namespace"], Name: alert.Labels["resource_name"], UID: alert.Labels["resource_uid"]},
		Labels:   alert.Labels, Message: alert.Annotations["description"], OccurredAt: occurredAt, ObservedAt: observedAt,
	}
	return Normalize(s), nil
}
