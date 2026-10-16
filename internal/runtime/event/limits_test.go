package event

import "testing"

func TestDeduperTTLsAndKeysByEventClass(t *testing.T) {
	deduper := NewDeduper()
	tests := []struct {
		name string
		evt  RuntimeEvent
		want int64
	}{
		{"warning", RuntimeEvent{Type: TypeK8sWarning}, 120},
		{"normal", RuntimeEvent{Type: TypeK8sNormal}, 30},
		{"alert firing", RuntimeEvent{Type: TypeAlertTriggered}, 15},
		{"alert resolved", RuntimeEvent{Type: TypeAlertResolved}, 15},
		{"unknown", RuntimeEvent{}, 60},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := int64(deduper.ttl(test.evt).Seconds()); got != test.want {
				t.Fatalf("ttl=%d want=%d", got, test.want)
			}
		})
	}
	base := RuntimeEvent{Type: TypeK8sWarning, Namespace: "ns", Resource: RuntimeResource{Kind: "Pod", Name: "pod"}, Message: "message"}
	if deduper.Seen(base) || !deduper.Seen(base) {
		t.Fatal("first event must be accepted and its duplicate rejected")
	}
	changed := base
	changed.Message = "other message"
	if deduper.Seen(changed) {
		t.Fatal("events with different messages must not share a dedupe key")
	}
}

func TestRateLimiterBoundariesByEventClass(t *testing.T) {
	tests := []struct {
		name  string
		type_ Type
		limit int
	}{
		{"warning", TypeK8sWarning, 10},
		{"normal", TypeK8sNormal, 5},
		{"alert", TypeAlertTriggered, 30},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter := NewRateLimiter()
			evt := RuntimeEvent{Type: test.type_, Namespace: "ns", Resource: RuntimeResource{Kind: "Pod", Name: "pod"}}
			for index := 0; index < test.limit; index++ {
				if !limiter.Allow(evt) {
					t.Fatalf("request %d of %d was rejected", index+1, test.limit)
				}
			}
			if limiter.Allow(evt) {
				t.Fatalf("request after limit %d was accepted", test.limit)
			}
		})
	}
}
