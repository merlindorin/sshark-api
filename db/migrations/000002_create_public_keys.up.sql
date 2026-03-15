CREATE TYPE key_type AS ENUM ('ssh', 'gpg');

CREATE TABLE public_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id   UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    key_type    key_type NOT NULL,
    key_data    BYTEA NOT NULL,
    fingerprint VARCHAR(64) UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_public_keys_source_id ON public_keys(source_id);
CREATE INDEX idx_public_keys_key_type ON public_keys(key_type);
CREATE INDEX idx_public_keys_fingerprint ON public_keys(fingerprint);