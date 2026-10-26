ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS template TEXT NOT NULL DEFAULT '';

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS empty_allowed BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS check_properties BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS check_commands BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS check_protections BOOLEAN NOT NULL DEFAULT false;
