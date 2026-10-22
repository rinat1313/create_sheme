CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY,
    login         TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL DEFAULT '',
    password_hash TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO users (id, login, display_name)
VALUES ('00000000-0000-0000-0000-000000000001', 'default', 'System Default')
ON CONFLICT (login) DO NOTHING;

CREATE TABLE IF NOT EXISTS user_solutions (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    role        TEXT NOT NULL DEFAULT 'owner',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, solution_id),
    CONSTRAINT user_solutions_role_chk CHECK (role IN ('owner', 'viewer'))
);

CREATE INDEX IF NOT EXISTS idx_user_solutions_user
    ON user_solutions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_solutions_solution
    ON user_solutions (solution_id);

INSERT INTO user_solutions (user_id, solution_id, role)
SELECT '00000000-0000-0000-0000-000000000001', s.id, 'owner'
FROM solutions s
ON CONFLICT DO NOTHING;

ALTER TABLE display_settings
    ADD COLUMN IF NOT EXISTS user_id UUID;

UPDATE display_settings
SET user_id = '00000000-0000-0000-0000-000000000001'
WHERE user_id IS NULL;

ALTER TABLE display_settings
    ALTER COLUMN user_id SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'display_settings_user_id_fkey'
    ) THEN
        ALTER TABLE display_settings
            ADD CONSTRAINT display_settings_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;

ALTER TABLE display_settings DROP CONSTRAINT IF EXISTS display_settings_pkey;
ALTER TABLE display_settings ADD PRIMARY KEY (user_id, template);

INSERT INTO display_settings (user_id, template, renderer, visible_fields, style) VALUES
    ('00000000-0000-0000-0000-000000000001', 'Pipe', 'pipe_strip', '[]', '{"labelPosition":"bottom","layout":"horizontal","strokeWidth":3}'),
    ('00000000-0000-0000-0000-000000000001', 'Bridge', 'pipe_strip', '[]', '{"labelPosition":"bottom","layout":"horizontal","strokeWidth":4}'),
    ('00000000-0000-0000-0000-000000000001', 'Pipeline', 'pipe_strip', '[]', '{"labelPosition":"bottom","layout":"horizontal","strokeWidth":3}'),
    ('00000000-0000-0000-0000-000000000001', 'Valve', 'valve_symbol', '[]', '{"labelPosition":"bottom","layout":"horizontal"}'),
    ('00000000-0000-0000-0000-000000000001', 'Tank', 'tank_barrel', '[]', '{"labelPosition":"bottom","layout":"horizontal"}'),
    ('00000000-0000-0000-0000-000000000001', 'PipelinePump', 'pump_symbol', '[]', '{"labelPosition":"bottom","layout":"horizontal"}')
ON CONFLICT (user_id, template) DO NOTHING;

ALTER TABLE compliance_rules
    ADD COLUMN IF NOT EXISTS user_id UUID,
    ADD COLUMN IF NOT EXISTS solution_id UUID REFERENCES solutions(id) ON DELETE CASCADE;

UPDATE compliance_rules
SET user_id = '00000000-0000-0000-0000-000000000001'
WHERE user_id IS NULL;

ALTER TABLE compliance_rules
    ALTER COLUMN user_id SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'compliance_rules_user_id_fkey'
    ) THEN
        ALTER TABLE compliance_rules
            ADD CONSTRAINT compliance_rules_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_compliance_rules_template;

CREATE INDEX IF NOT EXISTS idx_compliance_rules_user_template
    ON compliance_rules (user_id, template, created_at);

CREATE INDEX IF NOT EXISTS idx_compliance_rules_user_solution
    ON compliance_rules (user_id, solution_id, template)
    WHERE solution_id IS NOT NULL;

ALTER TABLE tag_edit_rules
    ADD COLUMN IF NOT EXISTS user_id UUID;

UPDATE tag_edit_rules
SET user_id = '00000000-0000-0000-0000-000000000001'
WHERE user_id IS NULL;

ALTER TABLE tag_edit_rules
    ALTER COLUMN user_id SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'tag_edit_rules_user_id_fkey'
    ) THEN
        ALTER TABLE tag_edit_rules
            ADD CONSTRAINT tag_edit_rules_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_tag_edit_rules_user_order
    ON tag_edit_rules (user_id, sort_order, created_at);
