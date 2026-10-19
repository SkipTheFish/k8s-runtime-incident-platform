package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App struct {
		Name            string
		Port            int
		ShutdownTimeout time.Duration
	}
	Postgres struct {
		DSN     string
		Timeout time.Duration
	}
	Redis struct {
		Addr string
	}
	PrometheusURL      string
	LokiURL            string
	TempoURL           string
	ClusterID          string
	TenantID           string
	KubernetesEnabled  bool
	HTTPRequestTimeout time.Duration
	HTTPMaxBodyBytes   int
	AdminToken         string
	DiagnosticsAddr    string
	DiagnosticsToken   string
	OTLPEndpoint       string
	OTELServiceName    string
	WebhookWorkers     int
	Webhook            struct {
		TargetName     string
		URL            string
		Secret         string
		Allowlist      []string
		ConnectTimeout time.Duration
		RequestTimeout time.Duration
		PollInterval   time.Duration
		RetryBase      time.Duration
		RetryMax       time.Duration
		MaxAttempts    int
		BatchSize      int
	}
	SignalWorkers    int
	SignalMaxRetries int
	SignalRetryBase  time.Duration
	SignalRetryMax   time.Duration
	ResourceGraph    struct {
		Retention       time.Duration
		CleanupInterval time.Duration
		CleanupTimeout  time.Duration
		CleanupBatch    int
	}
}

var Cfg *Config

func Init() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	Cfg = cfg
	return nil
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read configuration file: %w", err)
		}
	}
	return load(v.GetString)
}

func load(get func(string) string) (*Config, error) {
	cfg := &Config{}
	cfg.App.Name = valueOrDefault(get("APP_NAME"), "runtime-incident-platform")

	var err error
	if cfg.App.Port, err = parseInt("APP_PORT", get("APP_PORT"), 8080, 1, 65535); err != nil {
		return nil, err
	}
	if cfg.App.ShutdownTimeout, err = parseDuration("SHUTDOWN_TIMEOUT", get("SHUTDOWN_TIMEOUT"), 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.Postgres.Timeout, err = parseDuration("POSTGRES_TIMEOUT", get("POSTGRES_TIMEOUT"), 5*time.Second); err != nil {
		return nil, err
	}

	cfg.Postgres.DSN = strings.TrimSpace(get("POSTGRES_DSN"))
	if cfg.Postgres.DSN == "" {
		return nil, fieldError("POSTGRES_DSN", "is required")
	}
	cfg.Redis.Addr = valueOrDefault(get("REDIS_ADDR"), "127.0.0.1:6379")
	if !strings.Contains(cfg.Redis.Addr, ":") {
		return nil, fieldError("REDIS_ADDR", "must be in host:port form")
	}

	if cfg.PrometheusURL, err = parseHTTPURL("PROMETHEUS_URL", get("PROMETHEUS_URL"), "http://127.0.0.1:9090"); err != nil {
		return nil, err
	}
	if cfg.LokiURL, err = parseHTTPURL("LOKI_URL", get("LOKI_URL"), "http://127.0.0.1:3100"); err != nil {
		return nil, err
	}
	if cfg.TempoURL, err = parseHTTPURL("TEMPO_URL", get("TEMPO_URL"), "http://127.0.0.1:3200"); err != nil {
		return nil, err
	}

	cfg.ClusterID = strings.TrimSpace(get("CLUSTER_ID"))
	if cfg.ClusterID == "" {
		return nil, fieldError("CLUSTER_ID", "is required")
	}
	cfg.TenantID = valueOrDefault(get("TENANT_ID"), "default")
	if cfg.HTTPRequestTimeout, err = parseDuration("HTTP_REQUEST_TIMEOUT", get("HTTP_REQUEST_TIMEOUT"), 15*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPMaxBodyBytes, err = parseInt("HTTP_MAX_BODY_BYTES", get("HTTP_MAX_BODY_BYTES"), 1<<20, 1024, 16<<20); err != nil {
		return nil, err
	}
	cfg.AdminToken = get("ADMIN_TOKEN")
	cfg.DiagnosticsAddr = strings.TrimSpace(get("DIAGNOSTICS_ADDR"))
	cfg.DiagnosticsToken = get("DIAGNOSTICS_TOKEN")
	if cfg.DiagnosticsAddr != "" {
		if _, _, splitErr := net.SplitHostPort(cfg.DiagnosticsAddr); splitErr != nil {
			return nil, fieldError("DIAGNOSTICS_ADDR", "must be in host:port form")
		}
		if cfg.DiagnosticsToken == "" {
			return nil, fieldError("DIAGNOSTICS_TOKEN", "is required when DIAGNOSTICS_ADDR is configured")
		}
	}
	cfg.OTLPEndpoint = strings.TrimSpace(get("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if cfg.OTLPEndpoint != "" {
		if cfg.OTLPEndpoint, err = parseHTTPURL("OTEL_EXPORTER_OTLP_ENDPOINT", cfg.OTLPEndpoint, ""); err != nil {
			return nil, err
		}
	}
	cfg.OTELServiceName = valueOrDefault(get("OTEL_SERVICE_NAME"), cfg.App.Name)
	if cfg.KubernetesEnabled, err = parseBool("KUBERNETES_ENABLED", get("KUBERNETES_ENABLED"), true); err != nil {
		return nil, err
	}
	if cfg.WebhookWorkers, err = parseInt("WEBHOOK_WORKERS", get("WEBHOOK_WORKERS"), 2, 1, 64); err != nil {
		return nil, err
	}
	cfg.Webhook.TargetName = valueOrDefault(get("WEBHOOK_TARGET_NAME"), "incident-agent")
	cfg.Webhook.URL = strings.TrimSpace(get("WEBHOOK_URL"))
	cfg.Webhook.Secret = get("WEBHOOK_SECRET")
	if raw := strings.TrimSpace(get("WEBHOOK_ALLOWLIST")); raw != "" {
		for _, entry := range strings.Split(raw, ",") {
			if entry = strings.TrimSpace(entry); entry != "" {
				cfg.Webhook.Allowlist = append(cfg.Webhook.Allowlist, entry)
			}
		}
	}
	if cfg.Webhook.URL != "" {
		if _, err := parseHTTPURL("WEBHOOK_URL", cfg.Webhook.URL, ""); err != nil {
			return nil, err
		}
		if cfg.Webhook.Secret == "" {
			return nil, fieldError("WEBHOOK_SECRET", "is required when WEBHOOK_URL is configured")
		}
	}
	if cfg.Webhook.ConnectTimeout, err = parseDuration("WEBHOOK_CONNECT_TIMEOUT", get("WEBHOOK_CONNECT_TIMEOUT"), 3*time.Second); err != nil {
		return nil, err
	}
	if cfg.Webhook.RequestTimeout, err = parseDuration("WEBHOOK_REQUEST_TIMEOUT", get("WEBHOOK_REQUEST_TIMEOUT"), 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.Webhook.PollInterval, err = parseDuration("WEBHOOK_POLL_INTERVAL", get("WEBHOOK_POLL_INTERVAL"), time.Second); err != nil {
		return nil, err
	}
	if cfg.Webhook.RetryBase, err = parseDuration("WEBHOOK_RETRY_BASE", get("WEBHOOK_RETRY_BASE"), time.Second); err != nil {
		return nil, err
	}
	if cfg.Webhook.RetryMax, err = parseDuration("WEBHOOK_RETRY_MAX", get("WEBHOOK_RETRY_MAX"), time.Minute); err != nil {
		return nil, err
	}
	if cfg.Webhook.MaxAttempts, err = parseInt("WEBHOOK_MAX_ATTEMPTS", get("WEBHOOK_MAX_ATTEMPTS"), 5, 1, 100); err != nil {
		return nil, err
	}
	if cfg.Webhook.BatchSize, err = parseInt("WEBHOOK_BATCH_SIZE", get("WEBHOOK_BATCH_SIZE"), 20, 1, 1000); err != nil {
		return nil, err
	}
	if cfg.SignalWorkers, err = parseInt("SIGNAL_WORKERS", get("SIGNAL_WORKERS"), 2, 1, 64); err != nil {
		return nil, err
	}
	if cfg.SignalMaxRetries, err = parseInt("SIGNAL_MAX_RETRIES", get("SIGNAL_MAX_RETRIES"), 5, 0, 100); err != nil {
		return nil, err
	}
	if cfg.SignalRetryBase, err = parseDuration("SIGNAL_RETRY_BASE", get("SIGNAL_RETRY_BASE"), 100*time.Millisecond); err != nil {
		return nil, err
	}
	if cfg.SignalRetryMax, err = parseDuration("SIGNAL_RETRY_MAX", get("SIGNAL_RETRY_MAX"), 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.ResourceGraph.Retention, err = parseDuration("RESOURCE_GRAPH_RETENTION", get("RESOURCE_GRAPH_RETENTION"), 7*24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.ResourceGraph.CleanupInterval, err = parseDuration("RESOURCE_GRAPH_CLEANUP_INTERVAL", get("RESOURCE_GRAPH_CLEANUP_INTERVAL"), time.Hour); err != nil {
		return nil, err
	}
	if cfg.ResourceGraph.CleanupTimeout, err = parseDuration("RESOURCE_GRAPH_CLEANUP_TIMEOUT", get("RESOURCE_GRAPH_CLEANUP_TIMEOUT"), 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ResourceGraph.CleanupBatch, err = parseInt("RESOURCE_GRAPH_CLEANUP_BATCH", get("RESOURCE_GRAPH_CLEANUP_BATCH"), 100, 1, 10000); err != nil {
		return nil, err
	}
	return cfg, nil
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func parseDuration(field, value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fieldError(field, "must be a positive duration")
	}
	return parsed, nil
}

func parseInt(field, value string, fallback, min, max int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < min || parsed > max {
		return 0, fieldError(field, fmt.Sprintf("must be an integer from %d to %d", min, max))
	}
	return parsed, nil
}

func parseBool(field, value string, fallback bool) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fieldError(field, "must be a boolean")
	}
	return parsed, nil
}

func parseHTTPURL(field, value, fallback string) (string, error) {
	value = valueOrDefault(value, fallback)
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fieldError(field, "must be an absolute HTTP(S) URL")
	}
	return parsed.String(), nil
}

func fieldError(field, reason string) error {
	return fmt.Errorf("invalid configuration field %s: %s", field, reason)
}
