ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS parameter_scope TEXT NOT NULL DEFAULT 'object';

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS entity_template TEXT NOT NULL DEFAULT '';

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS parameter TEXT NOT NULL DEFAULT 'Tag';

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS value_source TEXT NOT NULL DEFAULT 'field';

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS actions JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE tag_edit_rules
SET
    parameter_scope = COALESCE(NULLIF(parameter_scope, ''), 'object'),
    parameter = CASE WHEN COALESCE(parameter, '') = '' THEN 'Tag' ELSE parameter END,
    value_source = CASE WHEN COALESCE(value_source, '') = '' THEN 'field' ELSE value_source END,
    actions = CASE
        WHEN actions IS NULL OR actions = '[]'::jsonb THEN
            jsonb_build_array(
                jsonb_build_object(
                    'parameter_scope', 'object',
                    'entity_template', '',
                    'parameter', 'Tag',
                    'value_source', 'field',
                    'action', action,
                    'replacement', replacement,
                    'replace_mode', 'found'
                )
            )
        ELSE actions
    END;

ALTER TABLE tag_edit_rules
    DROP CONSTRAINT IF EXISTS tag_edit_rules_text_condition_chk;

ALTER TABLE tag_edit_rules
    ADD CONSTRAINT tag_edit_rules_text_condition_chk
    CHECK (text_condition IN ('contains', 'starts_with', 'ends_with', 'empty'));

ALTER TABLE tag_edit_rules
    DROP CONSTRAINT IF EXISTS tag_edit_rules_parameter_scope_chk;

ALTER TABLE tag_edit_rules
    ADD CONSTRAINT tag_edit_rules_parameter_scope_chk
    CHECK (parameter_scope IN ('object', 'property', 'command', 'protection'));

ALTER TABLE tag_edit_rules
    DROP CONSTRAINT IF EXISTS tag_edit_rules_value_source_chk;

ALTER TABLE tag_edit_rules
    ADD CONSTRAINT tag_edit_rules_value_source_chk
    CHECK (value_source IN ('field', 'tag'));
