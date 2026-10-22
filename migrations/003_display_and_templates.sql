UPDATE display_settings SET visible_fields = '[]'::jsonb;

UPDATE display_settings SET style = style || '{"strokeWidth":3}'::jsonb WHERE template IN ('Pipe', 'Pipeline');
UPDATE display_settings SET style = '{"labelPosition":"bottom","layout":"horizontal","strokeWidth":4}'::jsonb WHERE template = 'Bridge';

CREATE TABLE IF NOT EXISTS solution_templates (
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    template TEXT NOT NULL,
    object_count INT NOT NULL DEFAULT 0,
    PRIMARY KEY (solution_id, template)
);

CREATE INDEX IF NOT EXISTS idx_solution_templates_solution ON solution_templates(solution_id);
