-- +goose Up
-- +goose StatementBegin
ALTER TABLE agents ADD COLUMN IF NOT EXISTS version TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents DROP COLUMN IF EXISTS version;
-- +goose StatementEnd
