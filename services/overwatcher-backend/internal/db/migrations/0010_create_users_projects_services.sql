-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      VARCHAR(255) NOT NULL UNIQUE,
    name       VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    compose_file VARCHAR(512) NOT NULL,
    environment  VARCHAR(64) NOT NULL DEFAULT 'production',
    enabled      BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id);

CREATE TABLE IF NOT EXISTS services (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    repo           VARCHAR(512) NOT NULL,
    root_directory VARCHAR(512) NOT NULL DEFAULT '/',
    branch         VARCHAR(255) NOT NULL DEFAULT 'main',
    image          VARCHAR(512) NOT NULL,
    tag            VARCHAR(255) NOT NULL DEFAULT 'latest',
    position       INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_services_project_id ON services(project_id);
CREATE INDEX IF NOT EXISTS idx_services_repo ON services(repo);

-- Additive: coexists with legacy deploy_mappings path until cutover.
ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_project_id ON agents(project_id)
    WHERE project_id IS NOT NULL;

ALTER TABLE deploy_intents
    ADD COLUMN IF NOT EXISTS project_id UUID;
CREATE INDEX IF NOT EXISTS idx_deploy_intents_project_id ON deploy_intents(project_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_deploy_intents_project_id;
ALTER TABLE deploy_intents DROP COLUMN IF EXISTS project_id;

DROP INDEX IF EXISTS idx_agents_project_id;
ALTER TABLE agents DROP COLUMN IF EXISTS project_id;

DROP INDEX IF EXISTS idx_services_repo;
DROP INDEX IF EXISTS idx_services_project_id;
DROP TABLE IF EXISTS services;

DROP INDEX IF EXISTS idx_projects_user_id;
DROP TABLE IF EXISTS projects;

DROP TABLE IF EXISTS users;
-- +goose StatementEnd
