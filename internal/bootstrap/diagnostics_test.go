package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"observability-platform/internal/config"
	"observability-platform/internal/correlation"
	"observability-platform/internal/handler"
)

func TestDiagnosticsRequiresBearerToken(t *testing.T) {
	handler := newDiagnosticsServer("127.0.0.1:0", "diagnostic-secret").Handler
	for _, test := range []struct {
		name, token string
		want        int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "wrong", http.StatusUnauthorized},
		{"authorized", "diagnostic-secret", http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d", response.Code, test.want)
			}
		})
	}
}

func TestPublicRouterDoesNotExposePprof(t *testing.T) {
	cfg := &config.Config{PrometheusURL: "http://prometheus", LokiURL: "http://loki", TempoURL: "http://tempo", TenantID: "default"}
	cfg.Redis.Addr = "redis:6379"
	cfg.Postgres.Timeout = time.Second
	cfg.HTTPRequestTimeout = time.Second
	cfg.HTTPMaxBodyBytes = 1 << 20
	readiness := handler.NewReadiness(func(context.Context) error { return nil }, false, time.Second)
	router, _, _, _ := buildRouter(cfg, readiness, correlation.NewRegistry())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("public pprof status=%d want=404", response.Code)
	}
}
