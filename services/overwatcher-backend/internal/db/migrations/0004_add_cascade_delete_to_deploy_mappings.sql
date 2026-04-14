-- +goose Up
-- +goose StatementBegin
ALTER TABLE deploy_mappings
    DROP CONSTRAINT deploy_mappings_agent_id_fkey,
    ADD CONSTRAINT deploy_mappings_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE deploy_mappings
    DROP CONSTRAINT deploy_mappings_agent_id_fkey,
    ADD CONSTRAINT deploy_mappings_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agents(id);
-- +goose StatementEnd
