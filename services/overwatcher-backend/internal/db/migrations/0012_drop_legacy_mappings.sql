-- +goose Up
-- +goose StatementBegin
-- Webhook cutover is complete; project_id carries everything stack_index
-- used to. Drop the legacy unique, then the column.
ALTER TABLE deploy_intents DROP CONSTRAINT IF EXISTS deploy_intents_delivery_id_stack_index_key;
ALTER TABLE deploy_intents DROP COLUMN IF EXISTS stack_index;

-- compose_file is now a property of the project, carried on each intent.
ALTER TABLE agents DROP COLUMN IF EXISTS compose_file;

-- Legacy repo→stack index, superseded by projects + services.
DROP TABLE IF EXISTS deploy_mapping_services;
DROP TABLE IF EXISTS deploy_mappings;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Drops are not reversible without data loss. Restore from backup if needed.
SELECT 1;
-- +goose StatementEnd
