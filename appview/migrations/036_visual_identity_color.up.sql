-- Color index for search-by-color: one row per palette entry of a visual
-- identity. lab is CIELab (D65); L2 distance approximates ΔE76.
CREATE TABLE visual_identity_color (
    visual_identity_id UUID      NOT NULL REFERENCES visual_identity(id) ON DELETE CASCADE,
    rank               SMALLINT  NOT NULL,
    lab                VECTOR(3) NOT NULL,
    fraction           REAL      NOT NULL,
    PRIMARY KEY (visual_identity_id, rank)
);

CREATE INDEX idx_vic_lab ON visual_identity_color
    USING hnsw (lab vector_l2_ops)
    WITH (m = 16, ef_construction = 64);
