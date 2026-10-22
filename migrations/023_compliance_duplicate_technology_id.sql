ALTER TABLE compliance_duplicate_rows
    ADD COLUMN IF NOT EXISTS technology_object_id TEXT NOT NULL DEFAULT '';
