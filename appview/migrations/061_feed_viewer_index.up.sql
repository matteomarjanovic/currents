-- Feed source selection and per-candidate viewer lookups must not scan the save
-- heap. Unlike the library index, this also covers saves of other content types.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_save_author_blob_feed
    ON save (author_did, pds_blob_cid) INCLUDE (collection_uri, created_at, uri);
