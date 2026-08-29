-- +goose Up
CREATE TABLE runtime_resources (
    id BIGSERIAL PRIMARY KEY,
    cluster_id TEXT NOT NULL,
    uid TEXT NOT NULL,
    api_version TEXT NOT NULL,
    kind TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    labels JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at TIMESTAMPTZ,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (cluster_id, uid)
);
CREATE INDEX idx_runtime_resources_active ON runtime_resources (cluster_id, kind, namespace, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_runtime_resources_deleted ON runtime_resources (deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE resource_edges (
    id BIGSERIAL PRIMARY KEY,
    cluster_id TEXT NOT NULL,
    from_uid TEXT NOT NULL,
    to_uid TEXT NOT NULL,
    edge_type TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (cluster_id, from_uid, to_uid, edge_type),
    FOREIGN KEY (cluster_id, from_uid) REFERENCES runtime_resources(cluster_id, uid) ON DELETE CASCADE,
    FOREIGN KEY (cluster_id, to_uid) REFERENCES runtime_resources(cluster_id, uid) ON DELETE CASCADE
);
CREATE INDEX idx_resource_edges_from ON resource_edges (cluster_id, from_uid, edge_type);
CREATE INDEX idx_resource_edges_to ON resource_edges (cluster_id, to_uid, edge_type);

-- +goose Down
DROP TABLE IF EXISTS resource_edges;
DROP TABLE IF EXISTS runtime_resources;
