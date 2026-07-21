-- Index backing the visual_identity.canonical_save_uri FK (ON DELETE SET NULL).
-- Without it every DELETE FROM save seq-scans visual_identity in the RI
-- trigger, making each unsave O(visual_identity) inside the DeleteSave
-- transaction — and account deletion loops that per save.
CREATE INDEX idx_vi_canonical_save_uri ON visual_identity (canonical_save_uri);
