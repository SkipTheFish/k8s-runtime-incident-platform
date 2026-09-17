package correlation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"

	"github.com/gin-gonic/gin"
)

type staticSimulationReader struct{ inputs []SimulationInput }

func (r staticSimulationReader) LoadSimulationInputs(context.Context, []string, int) ([]SimulationInput, error) {
	return append([]SimulationInput(nil), r.inputs...), nil
}

func TestSimulatorGroupsInputsReturnsUnmatchedAndIsDeterministic(t *testing.T) {
	rule := NewImagePullBackOffRule(100)
	simulator := NewSimulator(NewRegistry(rule), staticSimulationReader{inputs: []SimulationInput{{Signal: signal.Signal{ID: "history", ClusterID: "c", Source: signal.SourceKubernetes, Reason: "ImagePullBackOff", Labels: map[string]string{"container": "api"}}, Root: resourcegraph.ResourceKey{UID: "deploy"}}}}, 10)
	request := SimulationRequest{Generation: 7, HistoricalSignalIDs: []string{"history"}, Signals: []SimulationInput{{Signal: signal.Signal{ID: "upload", ClusterID: "c", Source: signal.SourcePrometheus, Type: signal.TypeAlertResolved, Reason: "KubePodImagePullBackOff", Labels: map[string]string{"container": "api"}}, Root: resourcegraph.ResourceKey{UID: "deploy"}}, {Signal: signal.Signal{ID: "other", ClusterID: "c", Source: signal.SourceKubernetes, Reason: "FailedScheduling"}, Root: resourcegraph.ResourceKey{UID: "pod"}}}}
	first, err := simulator.Simulate(context.Background(), "image-pull-backoff", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := simulator.Simulate(context.Background(), "image-pull-backoff", request)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if !bytes.Equal(a, b) {
		t.Fatalf("non-deterministic results:\n%s\n%s", a, b)
	}
	if first.InputCount != 3 || len(first.Incidents) != 1 || len(first.Incidents[0].SignalIDs) != 2 || first.Incidents[0].Recoveries != 1 || len(first.Unmatched) != 1 {
		t.Fatalf("result=%#v", first)
	}
}

func TestSimulatorLimitsAndPolicyLookup(t *testing.T) {
	simulator := NewSimulator(NewRegistry(NewImagePullBackOffRule(1)), nil, 1)
	_, err := simulator.Simulate(context.Background(), "image-pull-backoff", SimulationRequest{Signals: make([]SimulationInput, 2)})
	if err != ErrSimulationLimit {
		t.Fatalf("error=%v", err)
	}
	_, err = simulator.Simulate(context.Background(), "missing", SimulationRequest{})
	if err != ErrSimulationPolicyNotFound {
		t.Fatalf("error=%v", err)
	}
}

func TestSimulationHandlerRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSimulationHandler(NewSimulator(NewRegistry(NewImagePullBackOffRule(1)), nil, 10))
	router.POST("/api/v1/correlation-policies/:name/simulate", handler.Simulate)
	body := `{"signals":[],"padding":"` + strings.Repeat("x", int(MaxSimulationBodyBytes)) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/correlation-policies/image-pull-backoff/simulate", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
