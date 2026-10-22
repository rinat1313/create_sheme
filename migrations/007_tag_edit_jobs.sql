CREATE TABLE IF NOT EXISTS tag_edit_jobs (
    id UUID PRIMARY KEY,
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    total_objects INT NOT NULL DEFAULT 0,
    processed_objects INT NOT NULL DEFAULT 0,
    changed_objects INT NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    CONSTRAINT tag_edit_jobs_status_chk CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);
CREATE INDEX IF NOT EXISTS idx_tag_edit_jobs_solution ON tag_edit_jobs (solution_id, created_at DESC);
