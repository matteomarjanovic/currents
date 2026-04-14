ALTER TABLE visual_identity DROP COLUMN cluster_id;
DROP TABLE cluster;
DROP INDEX idx_vi_umap_embedding;
ALTER TABLE visual_identity DROP COLUMN umap_embedding;
