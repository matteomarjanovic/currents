ALTER TABLE import_job
    ADD COLUMN board_pin_count INT NOT NULL DEFAULT 0,
    ADD COLUMN listed_count INT,
    ADD COLUMN unsupported_count INT NOT NULL DEFAULT 0,
    ADD COLUMN listing_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN phase TEXT NOT NULL DEFAULT '',
    ADD COLUMN previously_done INT NOT NULL DEFAULT 0;

UPDATE import_job SET board_pin_count = expected_count;
-- Older checkpoints could advance in the middle of a page. Re-enumerate
-- unfinished listings; image deduplication preserves their existing queue.
UPDATE import_job SET list_cursor='' WHERE status='listing';

-- Keep completed history even when a user moves or deletes the imported save.
-- It prevents a later repair from recreating that image in the old destination.
CREATE INDEX idx_import_item_image ON import_item(owner_did, md5(image_url));
