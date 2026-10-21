CREATE TABLE blob_enrichment_failure (
    blob_cid TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('failed', 'unsupported', 'invalid', 'source_unavailable')),
    error TEXT NOT NULL,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
