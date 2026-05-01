-- +goose Up
-- +goose StatementBegin
ALTER TABLE services
    ADD COLUMN IF NOT EXISTS workflow VARCHAR(255) NOT NULL DEFAULT '';

-- Lookup support for the workflow_run trigger path.
CREATE INDEX IF NOT EXISTS idx_services_repo_workflow ON services(LOWER(repo), workflow)
    WHERE workflow <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_services_repo_workflow;
ALTER TABLE services DROP COLUMN IF EXISTS workflow;
-- +goose StatementEnd
