package incident

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestContextV1Golden(t *testing.T) {
	root, _ := SanitizeJSON([]byte(`{"kind":"Deployment","name":"api","token":"secret"}`))
	value := ContextV1{Schema: ContextSchemaV1, Revision: 2, Incident: IncidentView{ID: 7, Title: "CrashLoopBackOff", Status: "open", Version: 2, RootResource: root}, RootResource: root, AffectedResources: []ResourceContext{}, Signals: []SignalContext{}, Timeline: []TimelineView{}, Evidence: []map[string]any{}, CorrelationDecision: []CorrelationView{}, RawDataLinks: []string{}, Truncated: map[string]bool{}}
	got, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/context_v1.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != strings.TrimSpace(string(want)) {
		t.Fatalf("context schema changed\ngot:  %s\nwant: %s", got, want)
	}
}

func TestPGContextAggregatesCommittedIncident(t *testing.T) {
	pool := incidentTestPool(t)
	defer pool.Close()
	truncateIncidentTables(t, pool)
	created, err := NewTransactionService(pool, 3*time.Second).Process(context.Background(), incidentSignal("context", time.Now().UTC()), incidentMatch())
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewPGQueryStoreWithPool(pool).Context(context.Background(), "default", created.IncidentID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Schema != ContextSchemaV1 || result.Revision != 1 || len(result.Signals) != 1 || len(result.Timeline) != 1 || len(result.CorrelationDecision) != 1 || len(result.RawDataLinks) != 1 || len(result.AffectedResources) != 1 {
		t.Fatalf("context=%+v", result)
	}
	if result.Signals[0].RawDataLink == "" || IncidentETag(created.IncidentID, result.Revision) == "" {
		t.Fatalf("context=%+v", result)
	}
}
