ALTER TABLE compliance_rules
    ADD COLUMN IF NOT EXISTS parameter_scope TEXT NOT NULL DEFAULT 'object',
    ADD COLUMN IF NOT EXISTS entity_template TEXT NOT NULL DEFAULT '';
