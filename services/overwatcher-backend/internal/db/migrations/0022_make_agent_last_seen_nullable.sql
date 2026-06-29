-- +goose Up
-- +goose StatementBegin
-- last_seen_at must be empty until the agent's first heartbeat; the old
-- DEFAULT NOW() seeded it at creation, which made a never-connected agent look
-- "connected" (age ≈ 0).
ALTER TABLE agents ALTER COLUMN last_seen_at DROP DEFAULT;
ALTER TABLE agents ALTER COLUMN last_seen_at DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE agents SET last_seen_at = NOW() WHERE last_seen_at IS NULL;
ALTER TABLE agents ALTER COLUMN last_seen_at SET DEFAULT NOW();
ALTER TABLE agents ALTER COLUMN last_seen_at SET NOT NULL;
-- +goose StatementEnd
