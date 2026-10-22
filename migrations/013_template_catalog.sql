CREATE TABLE IF NOT EXISTS solution_template_parameters (
    solution_id    UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    scope          TEXT NOT NULL,
    owner_template TEXT NOT NULL,
    parameters     TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (solution_id, scope, owner_template)
);

CREATE TABLE IF NOT EXISTS solution_template_entities (
    solution_id      UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    object_template  TEXT NOT NULL,
    scope            TEXT NOT NULL,
    entity_templates TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (solution_id, object_template, scope),
    CONSTRAINT solution_template_entities_scope_chk
        CHECK (scope IN ('command', 'protection'))
);

ALTER TABLE solutions
    ADD COLUMN IF NOT EXISTS index_version INTEGER NOT NULL DEFAULT 0;

ALTER TABLE solution_template_parameters
    DROP CONSTRAINT IF EXISTS solution_template_parameters_scope_chk;

ALTER TABLE solution_template_parameters
    ADD CONSTRAINT solution_template_parameters_scope_chk
        CHECK (scope IN ('object', 'property', 'command', 'protection'));
