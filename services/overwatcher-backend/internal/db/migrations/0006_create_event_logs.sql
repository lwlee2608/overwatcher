-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS event_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id   VARCHAR(255) NOT NULL,
    event_type    VARCHAR(100) NOT NULL,
    repo          VARCHAR(512) NOT NULL DEFAULT '',
    sender        VARCHAR(255) NOT NULL DEFAULT '',
    summary       TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_logs_created_at ON event_logs(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_event_logs_created_at;
DROP TABLE IF EXISTS event_logs;
-- +goose StatementEnd
