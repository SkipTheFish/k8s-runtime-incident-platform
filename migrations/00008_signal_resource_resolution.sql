-- +goose Up
ALTER TABLE signals
    ADD COLUMN resolution_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (resolution_status IN ('pending', 'matched', 'unmatched')),
    ADD COLUMN root_resource_uid TEXT,
    ADD COLUMN root_resource_kind TEXT,
    ADD COLUMN root_resource_namespace TEXT,
    ADD COLUMN root_resource_name TEXT,
    ADD COLUMN resolution_error TEXT;
CREATE INDEX idx_signals_resolution_status ON signals (resolution_status, observed_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_signals_resolution_status;
ALTER TABLE signals
    DROP COLUMN IF EXISTS resolution_error,
    DROP COLUMN IF EXISTS root_resource_name,
    DROP COLUMN IF EXISTS root_resource_namespace,
    DROP COLUMN IF EXISTS root_resource_kind,
    DROP COLUMN IF EXISTS root_resource_uid,
    DROP COLUMN IF EXISTS resolution_status;
