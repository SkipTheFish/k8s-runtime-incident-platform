-- +goose Up
CREATE TABLE IF NOT EXISTS observability_targets (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    service_name TEXT NOT NULL,
    selector JSONB NOT NULL DEFAULT '{}'::jsonb,
    metrics_enabled BOOLEAN NOT NULL DEFAULT true,
    logs_enabled BOOLEAN NOT NULL DEFAULT true,
    traces_enabled BOOLEAN NOT NULL DEFAULT true,
    scrape_path TEXT NOT NULL DEFAULT '/metrics',
    scrape_port INTEGER NOT NULL DEFAULT 8080,
    scrape_interval TEXT NOT NULL DEFAULT '30s',
    owner TEXT,
    environment TEXT,
    k8s_namespace TEXT NOT NULL,
    k8s_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (k8s_namespace, k8s_name)
);

CREATE TABLE IF NOT EXISTS observe_tenants (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    owner TEXT NOT NULL,
    team TEXT,
    environment TEXT,
    retention_days INTEGER NOT NULL DEFAULT 30,
    metrics_quota_series INTEGER NOT NULL DEFAULT 100000,
    logs_quota_gb INTEGER NOT NULL DEFAULT 50,
    traces_quota_gb INTEGER NOT NULL DEFAULT 20,
    alert_limit INTEGER NOT NULL DEFAULT 50,
    enabled BOOLEAN NOT NULL DEFAULT true,
    k8s_namespace TEXT NOT NULL,
    k8s_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (k8s_namespace, k8s_name)
);

CREATE TABLE IF NOT EXISTS operator_reconcile_history (
    id BIGSERIAL PRIMARY KEY,
    resource_kind TEXT NOT NULL,
    resource_namespace TEXT NOT NULL,
    resource_name TEXT NOT NULL,
    action TEXT NOT NULL,
    result TEXT NOT NULL,
    message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE alert_policies
ADD COLUMN IF NOT EXISTS severity TEXT DEFAULT 'warning',
ADD COLUMN IF NOT EXISTS enabled BOOLEAN DEFAULT true,
ADD COLUMN IF NOT EXISTS owner TEXT,
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

-- +goose Down
ALTER TABLE alert_policies
DROP COLUMN IF EXISTS updated_at,
DROP COLUMN IF EXISTS owner,
DROP COLUMN IF EXISTS enabled,
DROP COLUMN IF EXISTS severity;
DROP TABLE IF EXISTS operator_reconcile_history;
DROP TABLE IF EXISTS observe_tenants;
DROP TABLE IF EXISTS observability_targets;
