ALTER TABLE public_keys ADD COLUMN provider_key_id VARCHAR(255);

CREATE INDEX idx_public_keys_provider_key_id ON public_keys(provider_key_id);
