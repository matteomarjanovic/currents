ALTER TABLE collection ADD COLUMN canonical_embedding VECTOR(768);

CREATE INDEX idx_collection_embedding ON collection
    USING hnsw (canonical_embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
