-- +goose Up
ALTER TABLE incidents ADD COLUMN reopen_window_seconds BIGINT NOT NULL DEFAULT 1800 CHECK (reopen_window_seconds > 0);

-- +goose Down
ALTER TABLE incidents DROP COLUMN IF EXISTS reopen_window_seconds;
