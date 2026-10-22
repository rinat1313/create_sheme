CREATE TABLE IF NOT EXISTS compliance_rules (
    id UUID PRIMARY KEY,
    template TEXT NOT NULL,
    parameter TEXT NOT NULL,
    text_condition TEXT NOT NULL DEFAULT 'must_contain',
    expected_text TEXT NOT NULL DEFAULT '',
    empty_allowed BOOLEAN NOT NULL DEFAULT TRUE,
    indication JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_compliance_rules_template ON compliance_rules(template);
