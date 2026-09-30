-- +goose Up
ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_payload_size CHECK (octet_length(payload::TEXT) <= 262144);
CREATE INDEX idx_outbox_stale_delivery ON outbox_events (updated_at, id) WHERE status = 'delivering';

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_stale_delivery;
ALTER TABLE outbox_events DROP CONSTRAINT IF EXISTS outbox_payload_size;
