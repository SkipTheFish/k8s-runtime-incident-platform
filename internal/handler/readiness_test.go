package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		kubernetes  bool
		databaseErr error
		migrations  bool
		cacheSync   bool
		dependency  error
		wantReady   int
	}{
		{name: "ready without Kubernetes", migrations: true, wantReady: http.StatusOK},
		{name: "database unavailable", databaseErr: errors.New("down"), migrations: true, wantReady: http.StatusServiceUnavailable},
		{name: "migrations pending", wantReady: http.StatusServiceUnavailable},
		{name: "Kubernetes cache not synced", kubernetes: true, migrations: true, wantReady: http.StatusServiceUnavailable},
		{name: "Kubernetes cache synced", kubernetes: true, migrations: true, cacheSync: true, wantReady: http.StatusOK},
		{name: "core dependency unavailable", migrations: true, dependency: errors.New("down"), wantReady: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewReadiness(func(context.Context) error { return tt.databaseErr }, tt.kubernetes, time.Second)
			state.SetMigrationsReady(tt.migrations)
			state.SetCacheSynced(tt.cacheSync)
			if tt.dependency != nil {
				state.AddDependency("core", func(context.Context) error { return tt.dependency })
			}
			router := gin.New()
			router.GET("/livez", state.Live)
			router.GET("/readyz", state.Ready)

			ready := httptest.NewRecorder()
			router.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if ready.Code != tt.wantReady {
				t.Fatalf("readyz status = %d, want %d; body=%s", ready.Code, tt.wantReady, ready.Body.String())
			}
			live := httptest.NewRecorder()
			router.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/livez", nil))
			if live.Code != http.StatusOK {
				t.Fatalf("livez status = %d, want 200", live.Code)
			}
		})
	}
}
