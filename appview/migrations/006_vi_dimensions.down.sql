ALTER TABLE visual_identity
    DROP COLUMN canonical_save_uri,
    ADD COLUMN dominant_colors JSONB;

ALTER TABLE save
    DROP COLUMN width,
    DROP COLUMN height,
    DROP COLUMN dominant_colors;
