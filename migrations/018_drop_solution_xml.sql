ALTER TABLE solutions DROP COLUMN IF EXISTS original_xml;
ALTER TABLE solutions DROP COLUMN IF EXISTS current_xml;

UPDATE solutions SET data = '{}'::jsonb WHERE data IS NULL;

ALTER TABLE solutions ALTER COLUMN data SET DEFAULT '{}'::jsonb;
ALTER TABLE solutions ALTER COLUMN data SET NOT NULL;
