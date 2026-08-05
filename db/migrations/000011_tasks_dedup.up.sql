-- Identifies the work a task represents: the kind alone for a refresh, the kind and the key for
-- a revocation. Two tasks with the same key are the same request, not two.
ALTER TABLE tasks ADD COLUMN dedup_key VARCHAR(128) NOT NULL DEFAULT '';

UPDATE tasks SET dedup_key = kind WHERE dedup_key = '';

ALTER TABLE tasks ALTER COLUMN dedup_key DROP DEFAULT;

-- The database decides, not the application. Checking for an existing task and then inserting
-- races: two clicks landing together both see nothing and both insert. A partial unique index
-- lets only one unfinished task per user and request exist, however many arrive at once, while
-- leaving finished ones alone so the same work can be asked for again later.
CREATE UNIQUE INDEX idx_tasks_active_dedup
    ON tasks (clerk_user_id, dedup_key)
    WHERE status IN ('pending', 'running');
