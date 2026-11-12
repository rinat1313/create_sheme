CREATE TABLE IF NOT EXISTS user_name_display_mode (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    mode    TEXT NOT NULL DEFAULT 'user',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_name_display_mode_chk CHECK (mode IN ('user', 'tech', 'both'))
);

ALTER TABLE solution_template_parameters
    ADD COLUMN IF NOT EXISTS labels JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE solution_template_entities
    ADD COLUMN IF NOT EXISTS labels JSONB NOT NULL DEFAULT '{}'::jsonb;
