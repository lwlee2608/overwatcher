-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS agents (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL UNIQUE,
    compose_file  VARCHAR(512) NOT NULL,
    remote_ip     VARCHAR(45) NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deploy_mappings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo         VARCHAR(512) NOT NULL,
    agent_id     UUID NOT NULL REFERENCES agents(id),
    services     TEXT[] NOT NULL DEFAULT '{}',
    environment  VARCHAR(255) NOT NULL DEFAULT 'production',
    enabled      BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(repo, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_deploy_mappings_repo ON deploy_mappings(repo);
CREATE INDEX IF NOT EXISTS idx_deploy_mappings_agent_id ON deploy_mappings(agent_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_deploy_mappings_agent_id;
DROP INDEX IF EXISTS idx_deploy_mappings_repo;
DROP TABLE IF EXISTS deploy_mappings;
DROP TABLE IF EXISTS agents;
-- +goose StatementEnd
