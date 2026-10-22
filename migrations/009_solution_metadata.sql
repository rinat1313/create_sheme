CREATE TABLE IF NOT EXISTS solution_metadata (
    solution_id UUID PRIMARY KEY REFERENCES solutions(id) ON DELETE CASCADE,
    automations JSONB NOT NULL DEFAULT '[]',
    control_commands JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
