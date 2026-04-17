DROP INDEX IF EXISTS idx_save_content_nsid;

ALTER TABLE save
    DROP COLUMN IF EXISTS content_nsid;