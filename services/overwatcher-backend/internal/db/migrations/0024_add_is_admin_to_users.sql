-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;
-- Upgrades without bootstrap credentials would otherwise have no admin. Skip
-- passwordless rows such as the legacy admin@local, which cannot log in.
UPDATE users SET is_admin = true
WHERE id = (SELECT id FROM users WHERE password_hash <> '' ORDER BY created_at LIMIT 1)
  AND NOT EXISTS (SELECT 1 FROM users WHERE is_admin);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS is_admin;
-- +goose StatementEnd
