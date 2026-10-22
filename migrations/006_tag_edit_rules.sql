CREATE TABLE IF NOT EXISTS tag_edit_rules (
    id UUID PRIMARY KEY,
    text_condition TEXT NOT NULL,
    pattern TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    replacement TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tag_edit_rules_text_condition_chk CHECK (text_condition IN ('contains', 'starts_with', 'ends_with')),
    CONSTRAINT tag_edit_rules_action_chk CHECK (action IN ('delete', 'replace'))
);
