CREATE TABLE IF NOT EXISTS compliance_duplicate_jobs (
    id UUID PRIMARY KEY,
    solution_id UUID NOT NULL REFERENCES solutions(id) ON DELETE CASCADE,
    rule_id UUID NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    total_objects INT NOT NULL DEFAULT 0,
    processed_objects INT NOT NULL DEFAULT 0,
    duplicate_tag_count INT NOT NULL DEFAULT 0,
    violated_object_count INT NOT NULL DEFAULT 0,
    result JSONB,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    CONSTRAINT compliance_duplicate_jobs_status_chk CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);
CREATE INDEX IF NOT EXISTS idx_compliance_duplicate_jobs_solution ON compliance_duplicate_jobs (solution_id, created_at DESC);
