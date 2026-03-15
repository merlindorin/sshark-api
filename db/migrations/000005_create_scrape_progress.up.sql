CREATE TABLE scrape_progress (
    provider    VARCHAR(50) PRIMARY KEY,
    last_cursor VARCHAR(255) NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scrape_progress_updated_at ON scrape_progress(updated_at);
