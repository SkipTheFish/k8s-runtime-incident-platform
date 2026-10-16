package signal

import (
	"strings"
	"testing"
	"time"
)

func TestStableID(t *testing.T) {
	base := Signal{ClusterID: "c1", Source: SourceKubernetes, Type: TypeWarning, SourceEventKey: "event-1", Reason: "BackOff", Resource: ObjectReference{UID: "uid-1"}, OccurredAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))}
	first := Normalize(base)
	second := Normalize(base)
	if first.ID != second.ID {
		t.Fatal("same input produced different IDs")
	}
	for name, mutate := range map[string]func(*Signal){
		"cluster":      func(s *Signal) { s.ClusterID = "c2" },
		"uid":          func(s *Signal) { s.Resource.UID = "uid-2" },
		"source event": func(s *Signal) { s.SourceEventKey = "event-2" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if Normalize(changed).ID == first.ID {
				t.Fatal("identity change did not change ID")
			}
		})
	}
}

func TestStableIDIdentityMatrix(t *testing.T) {
	base := Signal{ClusterID: "cluster-a", Source: SourceKubernetes, Type: TypeWarning, SourceEventKey: "event-a", Reason: "BackOff", Resource: ObjectReference{UID: "pod-a"}, OccurredAt: time.Date(2026, 10, 1, 2, 3, 4, 500, time.UTC)}
	want := StableID(base)
	tests := []struct {
		name   string
		mutate func(*Signal)
	}{
		{"source", func(s *Signal) { s.Source = SourcePrometheus }},
		{"cluster", func(s *Signal) { s.ClusterID = "cluster-b" }},
		{"source event key", func(s *Signal) { s.SourceEventKey = "event-b" }},
		{"resource uid", func(s *Signal) { s.Resource.UID = "pod-b" }},
		{"reason", func(s *Signal) { s.Reason = "FailedScheduling" }},
		{"occurred at", func(s *Signal) { s.OccurredAt = s.OccurredAt.Add(time.Nanosecond) }},
		{"type", func(s *Signal) { s.Type = TypeAlertFiring }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := base
			test.mutate(&changed)
			if StableID(changed) == want {
				t.Fatalf("identity field %q did not change stable ID", test.name)
			}
		})
	}
}

func TestNormalizeCanonicalizesTimeAndBoundsLabelsDeterministically(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	instant := time.Date(2026, 10, 1, 10, 3, 4, 567, location)
	labels := make(map[string]string, maxLabels+2)
	resourceLabels := make(map[string]string, maxLabels+2)
	for index := 0; index < maxLabels+2; index++ {
		key := string(rune('a'+index)) + strings.Repeat("x", index%3)
		labels[key] = "signal"
		resourceLabels[key] = "resource"
	}
	got := Normalize(Signal{OccurredAt: instant, ObservedAt: instant, Labels: labels, Resource: ObjectReference{Labels: resourceLabels}})
	if got.OccurredAt.Location() != time.UTC || got.ObservedAt.Location() != time.UTC {
		t.Fatalf("times were not normalized to UTC: occurred=%v observed=%v", got.OccurredAt, got.ObservedAt)
	}
	if !got.OccurredAt.Equal(instant) || got.OccurredAt.Nanosecond() != 567 {
		t.Fatalf("instant or nanoseconds changed: got=%s want=%s", got.OccurredAt, instant)
	}
	if len(got.Labels) != maxLabels || len(got.Resource.Labels) != maxLabels {
		t.Fatalf("label bounds not applied: signal=%d resource=%d", len(got.Labels), len(got.Resource.Labels))
	}
}

func TestNormalizeBounds(t *testing.T) {
	labels := make(map[string]string)
	for i := 0; i < 100; i++ {
		labels[string(rune(i+32))] = "v"
	}
	s := Normalize(Signal{Message: string(make([]byte, 5000)), Labels: labels})
	if len(s.Message) != maxMessage || len(s.Labels) != maxLabels {
		t.Fatalf("bounds not applied: message=%d labels=%d", len(s.Message), len(s.Labels))
	}
}
