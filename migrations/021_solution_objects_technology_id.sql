ALTER TABLE solution_objects
    ADD COLUMN IF NOT EXISTS technology_object_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_solution_objects_technology_id_trgm
    ON solution_objects USING GIN (technology_object_id gin_trgm_ops);

UPDATE solution_objects
SET technology_object_id = COALESCE(technology_payload->0->>'Id', '')
WHERE technology_object_id = ''
  AND jsonb_typeof(technology_payload) = 'array'
  AND jsonb_array_length(technology_payload) > 0
  AND COALESCE(technology_payload->0->>'Id', '') <> '';
