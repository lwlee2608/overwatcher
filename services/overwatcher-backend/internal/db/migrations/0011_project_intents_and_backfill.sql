-- +goose Up
-- +goose StatementBegin
ALTER TABLE deploy_intents ADD COLUMN IF NOT EXISTS compose_file VARCHAR(512) NOT NULL DEFAULT '';

-- Partial unique so ON CONFLICT (delivery_id, project_id) dedupe works for
-- webhook redeliveries on the new path. Pre-cutover rows (project_id IS NULL)
-- keep their old (delivery_id, stack_index) dedupe.
CREATE UNIQUE INDEX IF NOT EXISTS idx_deploy_intents_delivery_project
    ON deploy_intents(delivery_id, project_id)
    WHERE project_id IS NOT NULL;

-- --- Backfill legacy deploy_mappings → projects + services ---
-- Idempotent: only runs when there is legacy data AND no bootstrap user yet.
-- A single bootstrap user ('admin@local') owns every backfilled project.
DO $$
DECLARE
    bootstrap_user_id UUID;
    mapping_row       RECORD;
    project_row_id    UUID;
    has_legacy        BOOLEAN;
BEGIN
    SELECT EXISTS (SELECT 1 FROM deploy_mappings) INTO has_legacy;
    IF NOT has_legacy THEN
        RETURN;
    END IF;

    -- Bootstrap user (idempotent via ON CONFLICT).
    INSERT INTO users (email, name)
    VALUES ('admin@local', 'Bootstrap Admin')
    ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
    RETURNING id INTO bootstrap_user_id;

    FOR mapping_row IN
        SELECT dm.id            AS mapping_id,
               dm.repo          AS repo,
               dm.agent_id      AS agent_id,
               dm.environment   AS environment,
               dm.enabled       AS enabled,
               dm.created_at    AS created_at,
               a.name           AS agent_name,
               a.compose_file   AS compose_file,
               a.project_id     AS current_project_id
        FROM deploy_mappings dm
        JOIN agents a ON a.id = dm.agent_id
    LOOP
        -- Skip agents already bound to a project (re-run safety).
        IF mapping_row.current_project_id IS NOT NULL THEN
            CONTINUE;
        END IF;

        INSERT INTO projects (user_id, name, description, compose_file, environment, enabled, created_at)
        VALUES (
            bootstrap_user_id,
            mapping_row.agent_name || '-' || mapping_row.environment,
            'Backfilled from deploy_mapping ' || mapping_row.mapping_id::text,
            mapping_row.compose_file,
            mapping_row.environment,
            mapping_row.enabled,
            mapping_row.created_at
        )
        ON CONFLICT (user_id, name) DO UPDATE SET updated_at = NOW()
        RETURNING id INTO project_row_id;

        -- Copy compose services for this mapping. root_directory='/' and
        -- branch='main' per the redesign doc's backfill rules.
        INSERT INTO services (project_id, name, repo, root_directory, branch, image, tag, position)
        SELECT project_row_id,
               s.name,
               mapping_row.repo,
               '/',
               'main',
               s.image,
               s.tag,
               s.position
        FROM deploy_mapping_services s
        WHERE s.mapping_id = mapping_row.mapping_id
        ON CONFLICT (project_id, name) DO NOTHING;

        -- Bind the agent to the project (1:1, enforced by the partial unique).
        UPDATE agents SET project_id = project_row_id WHERE id = mapping_row.agent_id;
    END LOOP;
END;
$$;

-- Populate deploy_intents.project_id for historical rows so the dashboard can
-- still group by project after cutover.
UPDATE deploy_intents di
SET project_id = a.project_id,
    compose_file = COALESCE(p.compose_file, di.compose_file)
FROM agents a
LEFT JOIN projects p ON p.id = a.project_id
WHERE di.stack = a.name
  AND di.project_id IS NULL
  AND a.project_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The Down path only reverses the schema changes. Backfilled project/service
-- rows stay — reversing data deletion is not safe.
DROP INDEX IF EXISTS idx_deploy_intents_delivery_project;
ALTER TABLE deploy_intents DROP COLUMN IF EXISTS compose_file;
-- +goose StatementEnd
