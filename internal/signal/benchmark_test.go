package signal

import (
	"testing"
	"time"
)

var benchmarkSignalResult Signal

func BenchmarkSignalFingerprint(b *testing.B) {
	input := Signal{
		ClusterID:      "benchmark-cluster",
		Source:         SourceKubernetes,
		Type:           TypeWarning,
		SourceEventKey: "benchmark-event",
		Reason:         "BackOff",
		Resource:       ObjectReference{UID: "benchmark-pod"},
		OccurredAt:     time.Unix(1790000000, 123).UTC(),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		input.ID = StableID(input)
	}
	benchmarkSignalResult = input
}

func BenchmarkEventNormalization(b *testing.B) {
	labels := make(map[string]string, 96)
	for index := 0; index < 96; index++ {
		labels[string(rune(0x100+index))] = "benchmark-value"
	}
	input := Signal{
		ClusterID:      "benchmark-cluster",
		Source:         SourceKubernetes,
		Type:           TypeWarning,
		SourceEventKey: "benchmark-event",
		Reason:         "BackOff",
		Message:        string(make([]byte, 8192)),
		Labels:         labels,
		Resource:       ObjectReference{UID: "benchmark-pod", Labels: labels},
		OccurredAt:     time.Unix(1790000000, 123).In(time.FixedZone("UTC+8", 8*60*60)),
		ObservedAt:     time.Unix(1790000001, 456).In(time.FixedZone("UTC+8", 8*60*60)),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		benchmarkSignalResult = Normalize(input)
	}
}
