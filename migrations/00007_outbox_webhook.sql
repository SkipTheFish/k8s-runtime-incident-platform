-- +goose Up
CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    incident_id INTEGER NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivering', 'delivered', 'dead_letter')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);
CREATE INDEX idx_outbox_due ON outbox_events (next_attempt_at, id) WHERE status = 'pending';
CREATE INDEX idx_outbox_dead_letter ON outbox_events (updated_at DESC) WHERE status = 'dead_letter';

CREATE TABLE webhook_deliveries (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES outbox_events(event_id) ON DELETE CASCADE,
    attempt INTEGER NOT NULL CHECK (attempt > 0),
    target_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('delivering', 'succeeded', 'retryable', 'failed')),
    response_status INTEGER,
    error_summary TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    replayed_by TEXT,
    replay_reason TEXT,
    UNIQUE (event_id, attempt)
);
CREATE INDEX idx_webhook_delivery_event ON webhook_deliveries (event_id, attempt DESC);
CREATE INDEX idx_webhook_delivery_retry ON webhook_deliveries (next_attempt_at) WHERE status = 'retryable';

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS outbox_events;
