-- +goose Up
-- +goose StatementBegin
ALTER TABLE deploy_mappings
    ADD COLUMN image VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN tag   VARCHAR(255) NOT NULL DEFAULT 'latest';

UPDATE deploy_mappings
SET image = 'ghcr.io/' || LOWER(repo)
WHERE image = '';

ALTER TABLE deploy_mappings ALTER COLUMN image DROP DEFAULT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE deploy_mappings
    DROP COLUMN IF EXISTS tag,
    DROP COLUMN IF EXISTS image;
-- +goose StatementEnd
