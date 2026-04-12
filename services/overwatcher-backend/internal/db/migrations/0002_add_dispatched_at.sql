-- +goose Up
ALTER TABLE deploy_intents ADD COLUMN dispatched_at TIMESTAMP;

-- +goose Down
ALTER TABLE deploy_intents DROP COLUMN IF EXISTS dispatched_at;
