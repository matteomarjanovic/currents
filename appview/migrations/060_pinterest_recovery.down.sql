DROP INDEX IF EXISTS idx_import_item_image;
ALTER TABLE import_job
    DROP COLUMN board_pin_count,
    DROP COLUMN listed_count,
    DROP COLUMN unsupported_count,
    DROP COLUMN listing_error,
    DROP COLUMN phase,
    DROP COLUMN previously_done;
