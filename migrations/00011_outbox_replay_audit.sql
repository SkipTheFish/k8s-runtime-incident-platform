-- +goose Up
CREATE TABLE outbox_replays (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES outbox_events(event_id) ON DELETE CASCADE,
    previous_attempt INTEGER NOT NULL CHECK (previous_attempt > 0),
    actor TEXT NOT NULL,
    reason TEXT NOT NULL,
    replayed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, previous_attempt)
);
CREATE INDEX idx_outbox_replays_time ON outbox_replays (replayed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS outbox_replays;
