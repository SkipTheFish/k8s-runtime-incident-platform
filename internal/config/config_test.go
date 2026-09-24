package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	secretDSN := "postgres://admin:do-not-log@db:5432/incidents"
	tests := []struct {
		name      string
		values    map[string]string
		wantError string
		check     func(*testing.T, *Config)
	}{
		{name: "defaults without Kubernetes", values: map[string]string{"POSTGRES_DSN": secretDSN, "CLUSTER_ID": "local", "KUBERNETES_ENABLED": "false"}, check: func(t *testing.T, cfg *Config) {
			if cfg.App.Port != 8080 || cfg.WebhookWorkers != 2 || cfg.KubernetesEnabled || cfg.TenantID != "default" || cfg.HTTPRequestTimeout != 15*time.Second || cfg.HTTPMaxBodyBytes != 1<<20 {
				t.Fatalf("unexpected defaults: %+v", cfg)
			}
		}},
		{name: "custom parsed values", values: map[string]string{"POSTGRES_DSN": secretDSN, "CLUSTER_ID": "prod-a", "APP_PORT": "9091", "WEBHOOK_WORKERS": "8", "SHUTDOWN_TIMEOUT": "25s", "POSTGRES_TIMEOUT": "1500ms", "PROMETHEUS_URL": "https://prometheus.example.com", "KUBERNETES_ENABLED": "true"}, check: func(t *testing.T, cfg *Config) {
			if cfg.App.Port != 9091 || cfg.WebhookWorkers != 8 || cfg.App.ShutdownTimeout != 25*time.Second {
				t.Fatalf("parsed values do not match: %+v", cfg)
			}
		}},
		{name: "missing DSN", values: map[string]string{"CLUSTER_ID": "local"}, wantError: "POSTGRES_DSN"},
		{name: "missing cluster", values: map[string]string{"POSTGRES_DSN": secretDSN}, wantError: "CLUSTER_ID"},
		{name: "invalid port", values: required(secretDSN, "APP_PORT", "70000"), wantError: "APP_PORT"},
		{name: "invalid workers", values: required(secretDSN, "WEBHOOK_WORKERS", "0"), wantError: "WEBHOOK_WORKERS"},
		{name: "invalid duration", values: required(secretDSN, "SHUTDOWN_TIMEOUT", "forever"), wantError: "SHUTDOWN_TIMEOUT"},
		{name: "invalid graph retention", values: required(secretDSN, "RESOURCE_GRAPH_RETENTION", "0s"), wantError: "RESOURCE_GRAPH_RETENTION"},
		{name: "invalid cleanup batch", values: required(secretDSN, "RESOURCE_GRAPH_CLEANUP_BATCH", "0"), wantError: "RESOURCE_GRAPH_CLEANUP_BATCH"},
		{name: "invalid URL", values: required(secretDSN, "LOKI_URL", "http://"), wantError: "LOKI_URL"},
		{name: "invalid boolean", values: required(secretDSN, "KUBERNETES_ENABLED", "sometimes"), wantError: "KUBERNETES_ENABLED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := load(func(key string) string { return tt.values[key] })
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
				if strings.Contains(err.Error(), secretDSN) || strings.Contains(err.Error(), "do-not-log") {
					t.Fatalf("configuration error leaked a secret: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("load failed: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func required(dsn, key, value string) map[string]string {
	return map[string]string{"POSTGRES_DSN": dsn, "CLUSTER_ID": "local", key: value}
}
