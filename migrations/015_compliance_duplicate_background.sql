CREATE TABLE IF NOT EXISTS compliance_tag_counts (
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    count INT NOT NULL DEFAULT 0,
    PRIMARY KEY (solution_id, tag),
    CONSTRAINT compliance_tag_counts_count_chk CHECK (count >= 0)
);

CREATE TABLE IF NOT EXISTS compliance_tag_object_tags (
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    object_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    contrib INT NOT NULL DEFAULT 1,
    PRIMARY KEY (solution_id, object_id, tag),
    CONSTRAINT compliance_tag_object_tags_contrib_chk CHECK (contrib > 0)
);
CREATE INDEX IF NOT EXISTS idx_compliance_tag_object_tags_tag
    ON compliance_tag_object_tags (solution_id, tag);

CREATE TABLE IF NOT EXISTS compliance_duplicate_rows (
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    object_id TEXT NOT NULL,
    object_name TEXT NOT NULL DEFAULT '',
    object_type TEXT NOT NULL DEFAULT '',
    scheme_xml_id TEXT NOT NULL DEFAULT '',
    scheme_name TEXT NOT NULL DEFAULT '',
    scope TEXT NOT NULL DEFAULT '',
    scope_label TEXT NOT NULL DEFAULT '',
    source_label TEXT NOT NULL DEFAULT '',
    property_id TEXT NOT NULL DEFAULT '',
    entity_id TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (solution_id, tag, object_id, scope, property_id, entity_id, source_label)
);
CREATE INDEX IF NOT EXISTS idx_compliance_duplicate_rows_solution
    ON compliance_duplicate_rows (solution_id);
CREATE INDEX IF NOT EXISTS idx_compliance_duplicate_rows_object
    ON compliance_duplicate_rows (solution_id, object_id);

CREATE TABLE IF NOT EXISTS compliance_duplicate_scanner (
    solution_id UUID PRIMARY KEY REFERENCES solutions(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    mode TEXT NOT NULL DEFAULT 'idle',
    cursor_offset INT NOT NULL DEFAULT 0,
    processed INT NOT NULL DEFAULT 0,
    total INT NOT NULL DEFAULT 0,
    rule_id UUID,
    session_id UUID,
    status TEXT NOT NULL DEFAULT 'idle',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT compliance_duplicate_scanner_mode_chk CHECK (mode IN ('full', 'incremental', 'idle')),
    CONSTRAINT compliance_duplicate_scanner_status_chk CHECK (status IN ('idle', 'running', 'paused', 'error', 'completed'))
);

CREATE TABLE IF NOT EXISTS compliance_duplicate_dirty (
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    object_id TEXT NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (solution_id, object_id)
);
CREATE INDEX IF NOT EXISTS idx_compliance_duplicate_dirty_priority
    ON compliance_duplicate_dirty (solution_id, priority DESC, created_at ASC);
