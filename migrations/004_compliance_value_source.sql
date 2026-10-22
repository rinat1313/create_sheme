ALTER TABLE compliance_rules
    ADD COLUMN IF NOT EXISTS value_source TEXT NOT NULL DEFAULT 'field';
