package bootstrap

import (
	"context"

	"observability-platform/internal/repository"
)

func runMigrations(ctx context.Context) error {
	_, err := repository.DB.Exec(ctx, `
CREATE TABLE IF NOT EXISTS alert_policies (
    id SERIAL PRIMARY KEY, tenant_id TEXT, name TEXT, query TEXT,
    threshold DOUBLE PRECISION, operator TEXT, duration TEXT,
    created_at TIMESTAMP DEFAULT NOW(), k8s_namespace TEXT, k8s_name TEXT
);
CREATE TABLE IF NOT EXISTS alert_instances (
    id SERIAL PRIMARY KEY, policy_id INT, status TEXT, value DOUBLE PRECISION,
    start_time TIMESTAMP, end_time TIMESTAMP
);
CREATE TABLE IF NOT EXISTS incidents (
    id SERIAL PRIMARY KEY, tenant_id TEXT NOT NULL, namespace TEXT NOT NULL,
    policy_name TEXT NOT NULL, alert_name TEXT, severity TEXT, status TEXT NOT NULL,
    started_at TIMESTAMP DEFAULT NOW(), resolved_at TIMESTAMP, updated_at TIMESTAMP DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_open_unique
ON incidents (tenant_id, namespace, policy_name) WHERE status = 'open';
CREATE TABLE IF NOT EXISTS runtime_timeline (
    id SERIAL PRIMARY KEY, incident_id INT, tenant_id TEXT, namespace TEXT,
    source TEXT, event_type TEXT, resource_kind TEXT, resource_name TEXT,
    policy_name TEXT, message TEXT, created_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS runtime_graph_nodes (
    id SERIAL PRIMARY KEY, node_key TEXT UNIQUE NOT NULL, kind TEXT NOT NULL,
    namespace TEXT, name TEXT NOT NULL, tenant_id TEXT, updated_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS runtime_graph_edges (
    id SERIAL PRIMARY KEY, from_key TEXT NOT NULL, to_key TEXT NOT NULL,
    edge_type TEXT NOT NULL, updated_at TIMESTAMP DEFAULT NOW(), UNIQUE(from_key, to_key, edge_type)
);
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS resource_kind TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS resource_name TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMP;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS suppressed_at TIMESTAMP;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS reopen_count INT DEFAULT 0;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMP DEFAULT NOW();
`)
	return err
}
