-- +goose Up
-- +goose StatementBegin
ALTER TABLE projects DROP CONSTRAINT projects_user_id_fkey;
ALTER TABLE projects ADD CONSTRAINT projects_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_user_id_fkey;
ALTER TABLE projects ADD CONSTRAINT projects_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd
