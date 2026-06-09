DROP INDEX IF EXISTS idx_review_item_disputed_pending;
ALTER TABLE review_item
    DROP COLUMN IF EXISTS disputed_at,
    DROP COLUMN IF EXISTS disputed;
