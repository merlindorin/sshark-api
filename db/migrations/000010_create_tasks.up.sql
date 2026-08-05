CREATE TYPE task_status AS ENUM ('pending', 'running', 'succeeded', 'failed');

-- What a user sees when they ask sshark to do something that takes a while. River owns the
-- queue and the retries; this owns the story told back to the person who pressed the button,
-- which outlives any individual attempt and is safe to expose.
CREATE TABLE tasks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_user_id VARCHAR(255) NOT NULL,
    kind          VARCHAR(64) NOT NULL,
    status        task_status NOT NULL DEFAULT 'pending',
    -- How far along, counted in whatever unit the task decided on: accounts for a refresh,
    -- keys for a revocation. Total is 0 until the task knows how much work it has.
    progress      INTEGER NOT NULL DEFAULT 0,
    total         INTEGER NOT NULL DEFAULT 0,
    -- A sentence describing what is happening right now, shown verbatim in the UI.
    message       TEXT,
    -- Task-specific outcome, kept loose so a new task kind needs no migration.
    result        JSONB,
    error         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Listing a user's recent tasks is the common read.
CREATE INDEX idx_tasks_user_created_at ON tasks (clerk_user_id, created_at DESC);

-- Finding whether a user already has one of these in flight, so pressing refresh twice does
-- not queue two scrapes.
CREATE INDEX idx_tasks_user_kind_status ON tasks (clerk_user_id, kind, status);
