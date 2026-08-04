-- Records which SShark account owns a scraped source. Ownership is proven by connecting the
-- provider account, so this is written from the connected accounts rather than by matching
-- names: sharing a username with someone is not evidence of being them.
ALTER TABLE sources ADD COLUMN profile_id UUID REFERENCES profiles(id) ON DELETE SET NULL;

CREATE INDEX idx_sources_profile_id ON sources(profile_id);
