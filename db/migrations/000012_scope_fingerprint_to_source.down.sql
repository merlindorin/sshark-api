-- Only reversible while no two sources publish the same key, which is the state this migration
-- exists to allow. Going back therefore fails unless those rows are removed first.
DROP INDEX IF EXISTS idx_public_keys_source_fingerprint;

ALTER TABLE public_keys ADD CONSTRAINT public_keys_fingerprint_key UNIQUE (fingerprint);
