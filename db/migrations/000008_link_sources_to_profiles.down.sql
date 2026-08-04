DROP INDEX IF EXISTS idx_sources_profile_id;

ALTER TABLE sources DROP COLUMN IF EXISTS profile_id;
