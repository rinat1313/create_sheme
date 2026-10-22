CREATE TABLE IF NOT EXISTS solutions (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    external_id TEXT NOT NULL DEFAULT '',
    original_xml BYTEA NOT NULL,
    current_xml BYTEA,
    import_status TEXT NOT NULL DEFAULT 'ready',
    is_dirty BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS solution_nodes (
    id UUID PRIMARY KEY,
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES solution_nodes(id) ON DELETE CASCADE,
    node_type TEXT NOT NULL,
    xml_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    tag TEXT NOT NULL DEFAULT '',
    sort_index INT NOT NULL DEFAULT 0,
    meta JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_solution_nodes_solution ON solution_nodes(solution_id);
CREATE INDEX IF NOT EXISTS idx_solution_nodes_parent ON solution_nodes(parent_id);

CREATE TABLE IF NOT EXISTS graphic_schemes (
    id UUID PRIMARY KEY,
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    xml_id TEXT NOT NULL,
    project_xml_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    template TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(solution_id, xml_id)
);

CREATE TABLE IF NOT EXISTS edit_sessions (
    id UUID PRIMARY KEY,
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    active_scheme_xml_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS draft_changes (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES edit_sessions(id) ON DELETE CASCADE,
    scheme_xml_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_xml_id TEXT NOT NULL,
    patch JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS working_changes (
    id UUID PRIMARY KEY,
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    scheme_xml_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_xml_id TEXT NOT NULL,
    patch JSONB NOT NULL,
    merged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS display_settings (
    template TEXT PRIMARY KEY,
    renderer TEXT NOT NULL DEFAULT 'default',
    visible_fields JSONB NOT NULL DEFAULT '[]',
    style JSONB NOT NULL DEFAULT '{}'
);

