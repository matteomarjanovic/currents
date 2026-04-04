-- Dimensions and color palette belong to the save (they describe the specific blob).
ALTER TABLE save
    ADD COLUMN width           INTEGER,
    ADD COLUMN height          INTEGER,
    ADD COLUMN dominant_colors JSONB;

-- visual_identity now references its canonical save directly instead of
-- duplicating the color data. ON DELETE SET NULL so a delete always goes
-- through Go's DeleteSave re-election logic; the FK going NULL is the
-- graceful fallback if a direct delete ever bypasses Go.
ALTER TABLE visual_identity
    ADD COLUMN canonical_save_uri TEXT REFERENCES save(uri) ON DELETE SET NULL,
    DROP COLUMN dominant_colors;
