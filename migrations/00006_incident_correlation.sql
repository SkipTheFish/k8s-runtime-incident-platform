-- +goose Up
ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS fingerprint TEXT,
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN IF NOT EXISTS root_resource JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS correlation_policy TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN IF NOT EXISTS policy_generation BIGINT NOT NULL DEFAULT 0;
UPDATE incidents SET fingerprint = 'legacy:' || id::TEXT WHERE fingerprint IS NULL OR fingerprint = '';
ALTER TABLE incidents ALTER COLUMN fingerprint SET NOT NULL;
CREATE UNIQUE INDEX idx_incidents_active_fingerprint
ON incidents (fingerprint) WHERE lower(status) IN ('open', 'acknowledged', 'reopened');
CREATE INDEX idx_incidents_status_updated ON incidents (status, updated_at DESC);

CREATE TABLE incident_signals (
    incident_id INTEGER NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    signal_id TEXT NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    attached_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, signal_id),
    UNIQUE (signal_id)
);

CREATE TABLE timeline_entries (
    id BIGSERIAL PRIMARY KEY,
    incident_id INTEGER NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    entry_type TEXT NOT NULL,
    actor TEXT NOT NULL DEFAULT 'system',
    message TEXT NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (incident_id, idempotency_key)
);
CREATE INDEX idx_timeline_incident_time ON timeline_entries (incident_id, occurred_at, id);

CREATE TABLE correlation_decisions (
    id BIGSERIAL PRIMARY KEY,
    incident_id INTEGER REFERENCES incidents(id) ON DELETE CASCADE,
    signal_id TEXT NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    rule_name TEXT NOT NULL,
    rule_version TEXT NOT NULL,
    policy_generation BIGINT NOT NULL,
    root_resource JSONB NOT NULL,
    fingerprint TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    matched_conditions JSONB NOT NULL DEFAULT '[]'::jsonb,
    reason TEXT NOT NULL,
    decided_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (signal_id, rule_name, rule_version, policy_generation)
);
CREATE INDEX idx_correlation_decisions_incident ON correlation_decisions (incident_id, decided_at);

-- +goose Down
DROP TABLE IF EXISTS correlation_decisions;
DROP TABLE IF EXISTS timeline_entries;
DROP TABLE IF EXISTS incident_signals;
DROP INDEX IF EXISTS idx_incidents_active_fingerprint;
ALTER TABLE incidents DROP COLUMN IF EXISTS policy_generation, DROP COLUMN IF EXISTS correlation_policy,
    DROP COLUMN IF EXISTS version, DROP COLUMN IF EXISTS root_resource,
    DROP COLUMN IF EXISTS category, DROP COLUMN IF EXISTS fingerprint;
