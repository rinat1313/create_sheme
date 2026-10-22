CREATE TABLE IF NOT EXISTS solution_type_schemas (
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    template    TEXT NOT NULL,
    schema      JSONB NOT NULL DEFAULT '{}',
    source      TEXT NOT NULL DEFAULT 'inferred',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (solution_id, template),
    CONSTRAINT solution_type_schemas_source_chk
        CHECK (source IN ('passport', 'standalone', 'inferred'))
);

CREATE TABLE IF NOT EXISTS solution_objects (
    solution_id        UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    graphic_object_id  TEXT NOT NULL,
    scheme_xml_id      TEXT NOT NULL,
    template           TEXT NOT NULL DEFAULT '',
    tag                TEXT NOT NULL DEFAULT '',
    name               TEXT NOT NULL DEFAULT '',
    target_keys        TEXT[] NOT NULL DEFAULT '{}',
    technology_payload JSONB NOT NULL DEFAULT '[]',
    PRIMARY KEY (solution_id, graphic_object_id)
);

CREATE INDEX IF NOT EXISTS idx_solution_objects_solution_template
    ON solution_objects (solution_id, template);

CREATE INDEX IF NOT EXISTS idx_solution_objects_target_keys
    ON solution_objects USING GIN (target_keys);

CREATE TABLE IF NOT EXISTS solution_command_targets (
    solution_id   UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    target_key    TEXT NOT NULL,
    entity_type   TEXT NOT NULL,
    entity_xml_id TEXT NOT NULL,
    PRIMARY KEY (solution_id, entity_type, entity_xml_id, target_key),
    CONSTRAINT solution_command_targets_entity_type_chk
        CHECK (entity_type IN ('control_command', 'automation'))
);

CREATE INDEX IF NOT EXISTS idx_solution_command_targets_lookup
    ON solution_command_targets (solution_id, target_key);

CREATE INDEX IF NOT EXISTS idx_solution_command_targets_entity
    ON solution_command_targets (solution_id, entity_type, entity_xml_id);
