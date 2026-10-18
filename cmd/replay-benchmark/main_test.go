package main

import "testing"

func TestSmokeReplayProducesBoundedReport(t *testing.T) {
	load := profiles["smoke"]
	report, err := runReplay("smoke", load, 2)
	if err != nil {
		t.Fatal(err)
	}
	if report.Nodes != load.Nodes || report.Pods != load.Pods || report.Signals != load.Events {
		t.Fatalf("report scale=%+v", report)
	}
	if report.UniqueIncidents != load.IncidentKeys || report.DuplicateSignals != load.Events-load.IncidentKeys {
		t.Fatalf("incidents=%d duplicates=%d", report.UniqueIncidents, report.DuplicateSignals)
	}
	if report.ThroughputPerSecond <= 0 || report.LatencyP99Micros < report.LatencyP50Micros || report.MaximumQueueDepth < 0 {
		t.Fatalf("invalid metrics=%+v", report)
	}
	if report.PostgreSQL.Measured || report.Webhook.Measured {
		t.Fatal("synthetic replay must not claim external measurements")
	}
}

func TestFullProfileMatchesRequiredLogicalScale(t *testing.T) {
	load := profiles["full"]
	if load.Nodes != 500 || load.Pods != 10_000 || load.Events != 100_000 || load.ActiveAlerts != 2_000 || load.IncidentKeys != 10_000 {
		t.Fatalf("full profile=%+v", load)
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	values := []int64{1, 2, 3, 4, 5}
	if percentile(values, 50) != 3 || percentile(values, 95) != 5 || percentile(nil, 99) != 0 {
		t.Fatal("unexpected percentile")
	}
}
