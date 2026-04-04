CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE visual_identity (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_blob_did TEXT,
    canonical_blob_cid TEXT,
    embedding          VECTOR(768),
    save_count         INTEGER     NOT NULL DEFAULT 0 CHECK (save_count >= 0),
    dominant_colors    JSONB,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vi_embedding ON visual_identity
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

CREATE INDEX idx_vi_active ON visual_identity (id)
    WHERE canonical_blob_did IS NOT NULL;

ALTER TABLE save
    ADD COLUMN visual_identity_id UUID REFERENCES visual_identity(id),
    ADD COLUMN quality_score      REAL;

CREATE INDEX idx_save_vi         ON save (visual_identity_id);
CREATE INDEX idx_save_vi_quality ON save (visual_identity_id, quality_score DESC NULLS LAST);
