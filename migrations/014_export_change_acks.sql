CREATE TABLE IF NOT EXISTS export_change_acks (
    solution_id    UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    entity_type    TEXT NOT NULL,
    entity_xml_id  TEXT NOT NULL,
    acked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (solution_id, entity_type, entity_xml_id)
);

CREATE INDEX IF NOT EXISTS idx_export_change_acks_solution
    ON export_change_acks (solution_id);
