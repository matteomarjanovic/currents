ALTER TABLE collection ADD COLUMN parent_uri TEXT;
CREATE INDEX idx_collection_parent ON collection(parent_uri);
