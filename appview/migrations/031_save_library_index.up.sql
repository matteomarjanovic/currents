-- Covering index for the whole-library view: dedup an actor's image saves by blob CID
-- (DISTINCT ON pds_blob_cid, newest first) without a sort. Powers
-- is.currents.feed.getLibrarySaves (organize mode's "My library" root).
CREATE INDEX idx_save_author_blob_created ON save (author_did, pds_blob_cid, created_at DESC);
