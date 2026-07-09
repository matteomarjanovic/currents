DROP INDEX idx_label_uri;
DROP INDEX idx_label_blob_cid;
CREATE INDEX idx_label_uri_active      ON label (uri)      WHERE neg = FALSE;
CREATE INDEX idx_label_blob_cid_active ON label (blob_cid) WHERE neg = FALSE;
