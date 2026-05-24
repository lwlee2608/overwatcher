-- +goose Up
-- +goose StatementBegin
ALTER TABLE agents ADD COLUMN IF NOT EXISTS agent_type TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents DROP COLUMN IF EXISTS agent_type;
-- +goose StatementEnd
