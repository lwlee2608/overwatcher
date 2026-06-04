-- +goose Up
-- +goose StatementBegin
-- Per-agent auth: token_hash holds sha256(token); the raw token is never
-- stored. installed_by_user_id is the human who provisioned the agent — used
-- to scope visibility of an unbound agent before it is bound to a project.
ALTER TABLE agents ADD COLUMN IF NOT EXISTS token_hash TEXT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS installed_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
-- Partial unique index: many rows may carry NULL (pre-token / not yet issued),
-- but every issued digest must be unique so lookup resolves one agent.
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_token_hash ON agents (token_hash) WHERE token_hash IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agents_token_hash;
ALTER TABLE agents DROP COLUMN IF EXISTS installed_by_user_id;
ALTER TABLE agents DROP COLUMN IF EXISTS token_hash;
-- +goose StatementEnd
