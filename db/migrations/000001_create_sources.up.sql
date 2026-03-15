CREATE TABLE sources (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider   VARCHAR(50) NOT NULL,
    user_id    VARCHAR(255) NOT NULL,
    username   VARCHAR(255) NOT NULL,
    uri        TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (provider, user_id)
);

CREATE INDEX idx_sources_provider ON sources(provider);
CREATE INDEX idx_sources_username ON sources(username);
