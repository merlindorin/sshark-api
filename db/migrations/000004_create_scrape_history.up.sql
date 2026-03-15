CREATE TABLE scrape_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id      UUID NOT NULL REFERENCES public_keys(id) ON DELETE CASCADE,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    success     BOOLEAN NOT NULL,
    error       TEXT,
    key_changed BOOLEAN DEFAULT FALSE
);

CREATE INDEX idx_scrape_history_key_id ON scrape_history(key_id);
CREATE INDEX idx_scrape_history_scraped_at ON scrape_history(scraped_at DESC);