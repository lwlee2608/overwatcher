-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_deploy_intents_created_at ON deploy_intents(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_deploy_intents_created_at;
-- +goose StatementEnd
