-- +goose Up
-- +goose StatementBegin
ALTER TABLE deploy_intents ADD COLUMN services_spec JSONB NOT NULL DEFAULT '[]';

-- Backfill historical intents: pair each service name with the intent's
-- single image/tag (that was the effective runtime pairing anyway). Empty
-- services[] becomes a single-element list with name=''.
UPDATE deploy_intents di SET services_spec = COALESCE(
    (SELECT jsonb_agg(jsonb_build_object('name', name, 'image', di.image, 'tag', di.tag))
     FROM unnest(CASE WHEN cardinality(di.services) = 0
                      THEN ARRAY['']::text[]
                      ELSE di.services
                 END) AS name),
    '[]'::jsonb
);

ALTER TABLE deploy_intents
    DROP COLUMN image,
    DROP COLUMN tag,
    DROP COLUMN services;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE deploy_intents
    ADD COLUMN image    VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN tag      VARCHAR(255) NOT NULL DEFAULT 'latest',
    ADD COLUMN services TEXT[] NOT NULL DEFAULT '{}';

UPDATE deploy_intents SET
    image = COALESCE(services_spec->0->>'image', ''),
    tag   = COALESCE(services_spec->0->>'tag', 'latest'),
    services = COALESCE(
        (SELECT array_agg(elem->>'name')
         FROM jsonb_array_elements(services_spec) elem
         WHERE elem->>'name' <> ''),
        '{}'
    );

ALTER TABLE deploy_intents DROP COLUMN services_spec;
-- +goose StatementEnd
