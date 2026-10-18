package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"observability-platform/internal/correlation"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"
)

type profile struct {
	Nodes, Pods, Events, ActiveAlerts, IncidentKeys int
}

var profiles = map[string]profile{
	"smoke": {Nodes: 5, Pods: 100, Events: 2_000, ActiveAlerts: 50, IncidentKeys: 200},
	"full":  {Nodes: 500, Pods: 10_000, Events: 100_000, ActiveAlerts: 2_000, IncidentKeys: 10_000},
}

type topology struct {
	Nodes []string `json:"nodes"`
	Pods  []string `json:"pods"`
}

type externalMetrics struct {
	Measured bool   `json:"measured"`
	Reason   string `json:"reason,omitempty"`
}

type report struct {
	Profile              string          `json:"profile"`
	GoVersion            string          `json:"goVersion"`
	GOOS                 string          `json:"goos"`
	GOARCH               string          `json:"goarch"`
	Workers              int             `json:"workers"`
	Nodes                int             `json:"nodes"`
	Pods                 int             `json:"pods"`
	Signals              int             `json:"signals"`
	ActiveAlerts         int             `json:"activeAlerts"`
	UniqueIncidents      int             `json:"uniqueIncidents"`
	DuplicateSignals     int             `json:"duplicateSignals"`
	DedupeRatio          float64         `json:"dedupeRatio"`
	ThroughputPerSecond  float64         `json:"throughputPerSecond"`
	LatencyP50Micros     int64           `json:"latencyP50Micros"`
	LatencyP95Micros     int64           `json:"latencyP95Micros"`
	LatencyP99Micros     int64           `json:"latencyP99Micros"`
	MaximumQueueDepth    int64           `json:"maximumQueueDepth"`
	BacklogRecoveryMilli int64           `json:"backlogRecoveryMillis"`
	WallTimeMillis       int64           `json:"wallTimeMillis"`
	AllocatedBytes       uint64          `json:"allocatedBytes"`
	PeakHeapBytes        uint64          `json:"peakHeapBytes"`
	CPU                  externalMetrics `json:"cpu"`
	PostgreSQL           externalMetrics `json:"postgresql"`
	Webhook              externalMetrics `json:"webhook"`
	Notes                []string        `json:"notes"`
}

type workItem struct {
	index    int
	enqueued time.Time
}

func main() {
	profileName := flag.String("profile", "smoke", "workload profile: smoke or full")
	workers := flag.Int("workers", runtime.GOMAXPROCS(0), "correlation worker count")
	output := flag.String("output", "", "optional JSON report path")
	flag.Parse()
	load, ok := profiles[*profileName]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown profile %q\n", *profileName)
		os.Exit(2)
	}
	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "workers must be positive")
		os.Exit(2)
	}
	result, err := runReplay(*profileName, load, *workers)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	payload = append(payload, '\n')
	if *output != "" {
		if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(*output, payload, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	_, _ = os.Stdout.Write(payload)
}

func runReplay(name string, load profile, workers int) (report, error) {
	graph := buildTopology(load)
	registry := correlation.NewRegistry(correlation.NewImagePullBackOffRule(100), correlation.NewCrashLoopBackOffRule(90), correlation.NewFailedSchedulingRule(80))
	queue := make(chan workItem, min(load.Events, 4096))
	latencies := make([]int64, load.Events)
	fingerprints := make(map[string]struct{}, load.IncidentKeys)
	var fingerprintMu sync.Mutex
	var maximumDepth atomic.Int64
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	started := time.Now()
	monitorDone := make(chan struct{})
	var peakHeap atomic.Uint64
	peakHeap.Store(before.HeapAlloc)
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var sample runtime.MemStats
				runtime.ReadMemStats(&sample)
				for current := peakHeap.Load(); sample.HeapAlloc > current && !peakHeap.CompareAndSwap(current, sample.HeapAlloc); current = peakHeap.Load() {
				}
			case <-monitorDone:
				return
			}
		}
	}()
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for item := range queue {
				input := replayInput(item.index, load, graph)
				matched, err := registry.Evaluate(context.Background(), input)
				if err != nil {
					continue
				}
				fingerprintMu.Lock()
				fingerprints[matched.Fingerprint] = struct{}{}
				fingerprintMu.Unlock()
				latencies[item.index] = time.Since(item.enqueued).Microseconds()
			}
		}()
	}
	for index := 0; index < load.Events; index++ {
		queue <- workItem{index: index, enqueued: time.Now()}
		depth := int64(len(queue))
		for current := maximumDepth.Load(); depth > current && !maximumDepth.CompareAndSwap(current, depth); current = maximumDepth.Load() {
		}
	}
	close(queue)
	drainStarted := time.Now()
	group.Wait()
	close(monitorDone)
	recovery := time.Since(drainStarted)
	elapsed := time.Since(started)
	runtime.ReadMemStats(&after)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	unique := len(fingerprints)
	duplicates := load.Events - unique
	result := report{
		Profile: name, GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Workers: workers,
		Nodes: len(graph.Nodes), Pods: len(graph.Pods), Signals: load.Events, ActiveAlerts: load.ActiveAlerts,
		UniqueIncidents: unique, DuplicateSignals: duplicates, DedupeRatio: float64(duplicates) / float64(load.Events),
		ThroughputPerSecond: float64(load.Events) / elapsed.Seconds(), LatencyP50Micros: percentile(latencies, 50), LatencyP95Micros: percentile(latencies, 95), LatencyP99Micros: percentile(latencies, 99),
		MaximumQueueDepth: maximumDepth.Load(), BacklogRecoveryMilli: recovery.Milliseconds(), WallTimeMillis: elapsed.Milliseconds(), AllocatedBytes: after.TotalAlloc - before.TotalAlloc, PeakHeapBytes: peakHeap.Load(),
		CPU:        externalMetrics{Measured: false, Reason: "portable Go replay records wall time; process CPU peak requires the release environment collector"},
		PostgreSQL: externalMetrics{Measured: false, Reason: "synthetic replay does not connect to PostgreSQL"},
		Webhook:    externalMetrics{Measured: false, Reason: "synthetic replay does not deliver external webhooks"},
		Notes:      []string{"full profile models 100k signals as the daily logical volume", "CPU is represented by wall time and Go runtime allocation data; production CPU/DB/Webhook metrics require the release environment"},
	}
	return result, nil
}

func buildTopology(load profile) topology {
	result := topology{Nodes: make([]string, load.Nodes), Pods: make([]string, load.Pods)}
	for index := range result.Nodes {
		result.Nodes[index] = fmt.Sprintf("node-%04d", index)
	}
	for index := range result.Pods {
		result.Pods[index] = fmt.Sprintf("pod-%05d", index)
	}
	return result
}

func replayInput(index int, load profile, graph topology) correlation.EvaluationInput {
	key := index % load.IncidentKeys
	category := key % 3
	reason, message := "ImagePullBackOff", "Back-off pulling image"
	if category == 1 {
		reason, message = "BackOff", "Back-off restarting failed container"
	} else if category == 2 {
		reason, message = "FailedScheduling", "0/500 nodes are available"
	}
	pod := graph.Pods[key%len(graph.Pods)]
	rootUID := fmt.Sprintf("root-%05d", key)
	return correlation.EvaluationInput{
		Signal: signal.Signal{ClusterID: "replay", Source: signal.SourceKubernetes, Type: signal.TypeWarning, SourceEventKey: fmt.Sprintf("event-%d", index), Reason: reason, Message: message, Namespace: "benchmark", Resource: signal.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "benchmark", Name: pod, UID: fmt.Sprintf("pod-uid-%05d", key)}, Labels: map[string]string{"container": "app"}},
		Root:   resourcegraph.ResourceKey{ClusterID: "replay", APIVersion: "apps/v1", Kind: "Deployment", Namespace: "benchmark", Name: fmt.Sprintf("workload-%05d", key), UID: rootUID},
	}
}

func percentile(values []int64, p int) int64 {
	if len(values) == 0 {
		return 0
	}
	index := (len(values)*p + 99) / 100
	if index < 1 {
		index = 1
	}
	return values[index-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
