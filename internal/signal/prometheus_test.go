package signal

import (
	"testing"
	"time"
)

func TestFromPrometheusAlert(t *testing.T) {
	started := time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC)
	base := PrometheusAlert{Fingerprint: "fp-1", Status: "firing", StartsAt: started, Labels: map[string]string{"alertname": "CrashLoop", "namespace": "default", "resource_kind": "Pod", "resource_name": "pod-a", "resource_uid": "uid-a"}}
	first, err := FromPrometheusAlert("cluster", base, started.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	replay, _ := FromPrometheusAlert("cluster", base, started.Add(2*time.Minute))
	if first.ID != replay.ID || first.Type != TypeAlertFiring {
		t.Fatal("firing replay is not stable")
	}
	base.Status, base.EndsAt = "resolved", started.Add(5*time.Minute)
	resolved, err := FromPrometheusAlert("cluster", base, started.Add(6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Type != TypeAlertResolved || resolved.ID == first.ID {
		t.Fatal("resolved alert not distinguished")
	}
}

func TestPrometheusAlertMissingLabel(t *testing.T) {
	_, err := FromPrometheusAlert("cluster", PrometheusAlert{Fingerprint: "fp", Status: "firing", StartsAt: time.Now(), Labels: map[string]string{}}, time.Now())
	if err == nil {
		t.Fatal("expected missing label error")
	}
}
