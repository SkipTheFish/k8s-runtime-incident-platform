package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type openAPIDocument struct {
	OpenAPI    string                    `yaml:"openapi"`
	Paths      map[string]map[string]any `yaml:"paths"`
	Components map[string]any            `yaml:"components"`
}

func TestOpenAPIParsesAndMatchesRequiredRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document openAPIDocument
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}
	if document.OpenAPI != "3.0.3" || document.Components == nil {
		t.Fatalf("invalid document metadata: %+v", document)
	}
	for _, route := range RequiredRoutes() {
		path := strings.ReplaceAll(strings.ReplaceAll(route.Path, ":id", "{id}"), ":name", "{name}")
		operations, exists := document.Paths[path]
		if !exists || operations[strings.ToLower(route.Method)] == nil {
			t.Errorf("OpenAPI missing %s %s", route.Method, path)
		}
	}
}

func TestDocumentedExampleRoutesRunOnTestServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	for _, route := range RequiredRoutes() {
		router.Handle(route.Method, route.Path, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"requestID": "example"}) })
	}
	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/incidents?pageSize=10"},
		{http.MethodGet, "/api/v1/incidents/42"},
		{http.MethodGet, "/api/v1/incidents/42/timeline"},
		{http.MethodGet, "/api/v1/incidents/42/correlation"},
		{http.MethodGet, "/api/v1/incidents/42/context"},
		{http.MethodPost, "/api/v1/incidents/42/acknowledge"},
		{http.MethodPost, "/api/v1/correlation-policies/crash-loop-backoff/simulate"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("%s %s status=%d", request.method, request.path, response.Code)
		}
	}
}
