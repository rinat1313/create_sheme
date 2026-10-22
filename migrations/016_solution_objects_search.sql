CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_solution_objects_name_trgm
    ON solution_objects USING GIN (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_solution_objects_tag_trgm
    ON solution_objects USING GIN (tag gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_solution_objects_template_trgm
    ON solution_objects USING GIN (template gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_solution_objects_id_trgm
    ON solution_objects USING GIN (graphic_object_id gin_trgm_ops);
