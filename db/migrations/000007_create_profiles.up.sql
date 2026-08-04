-- citext gives a case-insensitive UNIQUE on username, so "Merlin" cannot be claimed
-- alongside "merlin" while the display keeps whatever casing the user chose.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE profiles (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_user_id VARCHAR(255) UNIQUE NOT NULL,
    username      CITEXT UNIQUE NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_profiles_username ON profiles(username);
