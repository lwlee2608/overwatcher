-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_is_bootstrap BOOLEAN NOT NULL DEFAULT false;
-- Existing rows came from the bootstrap path, so flag them as such. The flag
-- clears on the user's first ChangePassword call.
UPDATE users SET password_is_bootstrap = true WHERE password_hash <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS password_is_bootstrap;
-- +goose StatementEnd
