ALTER TABLE graphic_schemes
    ADD COLUMN IF NOT EXISTS payload_blob BYTEA,
    ADD COLUMN IF NOT EXISTS payload_encoding TEXT NOT NULL DEFAULT 'jsonb';

ALTER TABLE solutions
    ADD COLUMN IF NOT EXISTS data_blob BYTEA,
    ADD COLUMN IF NOT EXISTS data_encoding TEXT NOT NULL DEFAULT 'jsonb';
