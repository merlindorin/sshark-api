-- A fingerprint identifies a key, not a publication of one. The same public key is legitimately
-- published by more than one source: anyone who uploads one key to both GitHub and GitLab
-- publishes that fingerprint twice, and a person connecting a second provider is the ordinary
-- case rather than an edge one.
--
-- Held globally, the constraint let whichever provider was scraped first claim the fingerprint
-- and rejected every later one, so a user's GitLab keys silently never stored. Uniqueness
-- belongs to the pair: one row per key per source.
--
-- Lookups by fingerprint alone keep their own idx_public_keys_fingerprint, so dropping this
-- constraint costs no index the search depends on.
ALTER TABLE public_keys DROP CONSTRAINT public_keys_fingerprint_key;

CREATE UNIQUE INDEX idx_public_keys_source_fingerprint ON public_keys (source_id, fingerprint);
