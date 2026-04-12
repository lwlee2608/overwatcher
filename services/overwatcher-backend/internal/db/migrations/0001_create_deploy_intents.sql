-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS deploy_intents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id VARCHAR(255) NOT NULL,
    stack_index INTEGER NOT NULL DEFAULT 0,
    repo VARCHAR(512) NOT NULL,
    git_ref VARCHAR(255) NOT NULL,
    sha VARCHAR(64) NOT NULL,
    image VARCHAR(512) NOT NULL,
    tag VARCHAR(255) NOT NULL,
    stack VARCHAR(255) NOT NULL,
    services TEXT[] NOT NULL DEFAULT '{}',
    environment VARCHAR(255) NOT NULL,
    deployment_id BIGINT NOT NULL,
    installation_id BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'created',
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(delivery_id, stack_index)
);

CREATE INDEX IF NOT EXISTS idx_deploy_intents_status ON deploy_intents(status);
CREATE INDEX IF NOT EXISTS idx_deploy_intents_stack ON deploy_intents(stack);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_deploy_intents_stack;
DROP INDEX IF EXISTS idx_deploy_intents_status;
DROP TABLE IF EXISTS deploy_intents;
-- +goose StatementEnd
