CREATE TABLE IF NOT EXISTS solution_changed_objects (
    solution_id    UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    entity_type    TEXT NOT NULL,
    entity_xml_id  TEXT NOT NULL,
    scheme_xml_id  TEXT NOT NULL DEFAULT '',
    recorded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (solution_id, entity_type, entity_xml_id)
);

CREATE INDEX IF NOT EXISTS idx_solution_changed_objects_solution
    ON solution_changed_objects (solution_id);
