-- Refine 031's index into a covering partial index so getLibrarySaves' per-author
-- DISTINCT ON (pds_blob_cid) runs as an index-only scan — no seq scan + sort — even on
-- large libraries. The partial predicate matches the query's filters; uri is a trailing
-- key so the whole dedup+order is satisfied by the index and no heap access is needed.
DROP INDEX IF EXISTS idx_save_author_blob_created;
CREATE INDEX idx_save_author_blob_created ON save (author_did, pds_blob_cid, created_at DESC, uri)
	WHERE content_nsid = 'is.currents.content.image' AND pds_blob_cid <> '';
