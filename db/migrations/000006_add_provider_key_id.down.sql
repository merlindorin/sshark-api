DROP INDEX IF EXISTS idx_public_keys_provider_key_id;

ALTER TABLE public_keys DROP COLUMN IF EXISTS provider_key_id;
