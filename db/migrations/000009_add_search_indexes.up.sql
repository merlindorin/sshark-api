-- Every search orders by created_at and filters by key_type. Without an index covering both,
-- PostgreSQL sorted every matching row to return a page of 25 — an external merge spilling to
-- disk once the table outgrew work_mem, which is what made browsing take seconds.
CREATE INDEX idx_public_keys_type_created_at ON public_keys (key_type, created_at DESC);

-- Basic search wraps the term in wildcards (LIKE '%term%'), which a btree index cannot serve,
-- so looking up one person meant a sequential scan of every source. Trigrams index the
-- substrings, turning that into a bitmap index scan.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_sources_username_trgm ON sources USING gin (username gin_trgm_ops);
