-- The partial "active" label indexes (WHERE neg = FALSE) are never usable:
-- active-label queries must also read negation rows (the latest row per
-- (src, uri, val) decides activeness), so the predicate disqualifies them and
-- every label hydration seq-scans the table. Plain indexes serve those queries.
DROP INDEX idx_label_uri_active;
DROP INDEX idx_label_blob_cid_active;
CREATE INDEX idx_label_uri      ON label (uri);
CREATE INDEX idx_label_blob_cid ON label (blob_cid);
