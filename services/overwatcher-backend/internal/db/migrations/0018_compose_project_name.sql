-- +goose Up
-- +goose StatementBegin
-- compose_project_name is the immutable docker-compose --project-name slug.
-- Derived from projects.name once at create time, then frozen — renaming a
-- project must not orphan containers started under the old slug.
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS compose_project_name VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE deploy_intents
    ADD COLUMN IF NOT EXISTS compose_project_name VARCHAR(255) NOT NULL DEFAULT '';

-- Backfill existing projects. The regex mirrors the application-layer
-- slugify: lowercase, runs of non-[a-z0-9_-] collapse to '-', strip leading
-- and trailing dashes/underscores. If the result is empty (e.g. a name made
-- entirely of punctuation), fall back to a deterministic id-derived slug so
-- the column never holds ''.
UPDATE projects
SET compose_project_name = LOWER(
    REGEXP_REPLACE(
        REGEXP_REPLACE(name, '[^A-Za-z0-9_-]+', '-', 'g'),
        '^[-_]+|[-_]+$', '', 'g'
    )
)
WHERE compose_project_name = '';

UPDATE projects
SET compose_project_name = 'project-' || SUBSTRING(id::text FROM 1 FOR 8)
WHERE compose_project_name = '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE deploy_intents DROP COLUMN IF EXISTS compose_project_name;
ALTER TABLE projects       DROP COLUMN IF EXISTS compose_project_name;
-- +goose StatementEnd
