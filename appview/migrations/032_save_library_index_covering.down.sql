DROP INDEX IF EXISTS idx_save_author_blob_created;
CREATE INDEX idx_save_author_blob_created ON save (author_did, pds_blob_cid, created_at DESC);
