-- +goose Up
-- +goose StatementBegin
ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS cpu_percent REAL,
    ADD COLUMN IF NOT EXISTS mem_used_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS mem_total_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS disk_used_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS disk_total_bytes BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents
    DROP COLUMN IF EXISTS cpu_percent,
    DROP COLUMN IF EXISTS mem_used_bytes,
    DROP COLUMN IF EXISTS mem_total_bytes,
    DROP COLUMN IF EXISTS disk_used_bytes,
    DROP COLUMN IF EXISTS disk_total_bytes;
-- +goose StatementEnd
