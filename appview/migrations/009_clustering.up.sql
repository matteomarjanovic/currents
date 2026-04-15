ALTER TABLE visual_identity ADD COLUMN umap_embedding VECTOR(50);

CREATE INDEX idx_vi_umap_embedding ON visual_identity
    USING hnsw (umap_embedding vector_l2_ops) WITH (m = 16, ef_construction = 64);

CREATE TABLE cluster (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_date                  DATE NOT NULL,
    size                      INTEGER NOT NULL,
    medoid_visual_identity_id UUID REFERENCES visual_identity(id) DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX idx_cluster_run_date ON cluster (run_date);

ALTER TABLE visual_identity
    ADD COLUMN cluster_id UUID REFERENCES cluster(id) DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX idx_vi_cluster_id ON visual_identity (cluster_id);
