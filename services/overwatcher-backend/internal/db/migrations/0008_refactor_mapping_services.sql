-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS deploy_mapping_services (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mapping_id  UUID NOT NULL REFERENCES deploy_mappings(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    image       VARCHAR(512) NOT NULL,
    tag         VARCHAR(255) NOT NULL DEFAULT 'latest',
    position    INTEGER NOT NULL DEFAULT 0,
    UNIQUE(mapping_id, name)
);

CREATE INDEX IF NOT EXISTS idx_deploy_mapping_services_mapping_id
    ON deploy_mapping_services(mapping_id);

-- Backfill: expand each mapping's services[] into child rows carrying the
-- mapping's current image/tag. Mappings with an empty services[] get a
-- single child with name='' so the "apply to all compose services"
-- behavior is preserved.
INSERT INTO deploy_mapping_services (mapping_id, name, image, tag, position)
SELECT dm.id, svc.name, dm.image, dm.tag, svc.ord - 1
FROM deploy_mappings dm,
     LATERAL (
         SELECT s.name, s.ord
         FROM unnest(dm.services) WITH ORDINALITY AS s(name, ord)
         UNION ALL
         SELECT ''::text, 1::bigint WHERE cardinality(dm.services) = 0
     ) svc;

ALTER TABLE deploy_mappings
    DROP COLUMN services,
    DROP COLUMN image,
    DROP COLUMN tag;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE deploy_mappings
    ADD COLUMN services TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN image    VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN tag      VARCHAR(255) NOT NULL DEFAULT 'latest';

UPDATE deploy_mappings dm SET
    services = COALESCE(
        (SELECT array_agg(name ORDER BY position)
         FROM deploy_mapping_services
         WHERE mapping_id = dm.id AND name <> ''),
        '{}'
    ),
    image = COALESCE(
        (SELECT image FROM deploy_mapping_services
         WHERE mapping_id = dm.id ORDER BY position LIMIT 1),
        ''
    ),
    tag = COALESCE(
        (SELECT tag FROM deploy_mapping_services
         WHERE mapping_id = dm.id ORDER BY position LIMIT 1),
        'latest'
    );

DROP INDEX IF EXISTS idx_deploy_mapping_services_mapping_id;
DROP TABLE IF EXISTS deploy_mapping_services;
-- +goose StatementEnd
