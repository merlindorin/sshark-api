DROP INDEX IF EXISTS idx_tasks_active_dedup;

ALTER TABLE tasks DROP COLUMN IF EXISTS dedup_key;
