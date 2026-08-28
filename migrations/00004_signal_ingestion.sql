-- +goose Up
CREATE TABLE signals (
    id TEXT PRIMARY KEY,
    cluster_id TEXT NOT NULL,
    source TEXT NOT NULL,
    type TEXT NOT NULL,
    source_event_key TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    namespace TEXT NOT NULL DEFAULT '',
    resource_api_version TEXT NOT NULL DEFAULT '',
    resource_kind TEXT NOT NULL DEFAULT '',
    resource_name TEXT NOT NULL DEFAULT '',
    resource_uid TEXT NOT NULL DEFAULT '',
    owner_references JSONB NOT NULL DEFAULT '[]'::jsonb,
    labels JSONB NOT NULL DEFAULT '{}'::jsonb,
    message TEXT NOT NULL DEFAULT '',
    raw_reference JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, cluster_id, source_event_key)
);
CREATE INDEX idx_signals_source_occurred ON signals (source, occurred_at DESC);
CREATE INDEX idx_signals_resource_time ON signals (cluster_id, resource_uid, occurred_at DESC);

CREATE TABLE processed_events (
    id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL,
    cluster_id TEXT NOT NULL,
    source_event_key TEXT NOT NULL,
    signal_id TEXT NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, cluster_id, source_event_key)
);
CREATE INDEX idx_processed_events_time ON processed_events (processed_at DESC);

CREATE TABLE dead_letter_events (
    id BIGSERIAL PRIMARY KEY,
    signal_id TEXT REFERENCES signals(id) ON DELETE SET NULL,
    source TEXT NOT NULL,
    source_event_key TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_category TEXT NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL CHECK (attempts > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'replaying', 'resolved')),
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dead_letter_status ON dead_letter_events (status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS dead_letter_events;
DROP TABLE IF EXISTS processed_events;
DROP TABLE IF EXISTS signals;
