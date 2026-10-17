package signal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type Source string
type Type string

const (
	SourceKubernetes  Source = "kubernetes"
	SourcePrometheus  Source = "prometheus"
	TypeWarning       Type   = "warning"
	TypeNormal        Type   = "normal"
	TypeAlertFiring   Type   = "alert.firing"
	TypeAlertResolved Type   = "alert.resolved"
	maxMessage               = 4096
	maxLabels                = 64
)

type ObjectReference struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Namespace  string            `json:"namespace"`
	Name       string            `json:"name"`
	UID        string            `json:"uid"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type Signal struct {
	ID              string            `json:"id"`
	ClusterID       string            `json:"clusterID"`
	Source          Source            `json:"source"`
	Type            Type              `json:"type"`
	SourceEventKey  string            `json:"sourceEventKey"`
	Reason          string            `json:"reason"`
	Namespace       string            `json:"namespace"`
	Resource        ObjectReference   `json:"resource"`
	OwnerReferences []ObjectReference `json:"ownerReferences,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Message         string            `json:"message"`
	OccurredAt      time.Time         `json:"occurredAt"`
	ObservedAt      time.Time         `json:"observedAt"`
	RawReference    json.RawMessage   `json:"rawReference,omitempty"`
}

func Normalize(s Signal) Signal {
	s.OccurredAt = s.OccurredAt.UTC().Round(0)
	s.ObservedAt = s.ObservedAt.UTC().Round(0)
	if len(s.Message) > maxMessage {
		s.Message = s.Message[:maxMessage]
	}
	s.Labels = boundedLabels(s.Labels)
	s.Resource.Labels = boundedLabels(s.Resource.Labels)
	s.ID = StableID(s)
	return s
}

func StableID(s Signal) string {
	parts := []string{string(s.Source), s.ClusterID, s.SourceEventKey, s.Resource.UID, s.Reason, s.OccurredAt.UTC().Format(time.RFC3339Nano), string(s.Type)}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func boundedLabels(labels map[string]string) map[string]string {
	if len(labels) <= maxLabels {
		return labels
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string, maxLabels)
	for _, key := range keys[:maxLabels] {
		result[key] = labels[key]
	}
	return result
}
