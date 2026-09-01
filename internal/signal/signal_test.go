package signal

import (
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
