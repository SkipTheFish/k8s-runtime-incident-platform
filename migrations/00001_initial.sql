-- +goose Up
CREATE TABLE IF NOT EXISTS metrics_cache (
    id SERIAL PRIMARY KEY,
    tenant_id TEXT,
    query TEXT,
    result JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alerts (
    id SERIAL PRIMARY KEY,
    tenant_id TEXT,
    name TEXT,
    status TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alert_policies (
    id SERIAL PRIMARY KEY,
    tenant_id TEXT,
    name TEXT,
    query TEXT,
    threshold DOUBLE PRECISION,
    operator TEXT,
    duration TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alert_instances (
    id SERIAL PRIMARY KEY,
    policy_id INT,
    status TEXT,
    value DOUBLE PRECISION,
    start_time TIMESTAMP,
    end_time TIMESTAMP
);

CREATE TABLE IF NOT EXISTS incidents (
    id SERIAL PRIMARY KEY,
    tenant_id TEXT,
    title TEXT,
    status TEXT,
    start_time TIMESTAMP,
    end_time TIMESTAMP
);

CREATE TABLE IF NOT EXISTS timeline_events (
    id SERIAL PRIMARY KEY,
    incident_id INT,
    type TEXT,
    message TEXT,
    timestamp TIMESTAMP DEFAULT NOW()
);

ALTER TABLE alert_policies
ADD COLUMN IF NOT EXISTS k8s_namespace TEXT,
ADD COLUMN IF NOT EXISTS k8s_name TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_policies_k8s_ref
ON alert_policies(k8s_namespace, k8s_name)
WHERE k8s_namespace IS NOT NULL AND k8s_name IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS timeline_events;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS alert_instances;
DROP TABLE IF EXISTS alert_policies;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS metrics_cache;
