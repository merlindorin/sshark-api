CREATE TABLE ssh_key_metadata (
    key_id    UUID PRIMARY KEY REFERENCES public_keys(id) ON DELETE CASCADE,
    algorithm VARCHAR(50) NOT NULL,
    comment   TEXT DEFAULT '',
    options   TEXT[] DEFAULT '{}',
    key_bits  INTEGER
);

CREATE TABLE gpg_key_metadata (
    key_id       UUID PRIMARY KEY REFERENCES public_keys(id) ON DELETE CASCADE,
    algorithm    VARCHAR(50) NOT NULL,
    key_bits     INTEGER,
    expires_at   TIMESTAMPTZ,
    user_ids     TEXT[] DEFAULT '{}',
    capabilities TEXT[] DEFAULT '{}'
);

CREATE INDEX idx_ssh_metadata_algorithm ON ssh_key_metadata(algorithm);