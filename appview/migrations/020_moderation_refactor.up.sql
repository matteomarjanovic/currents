-- Add 'suspected' to the valid harm_state values (replaces unused 'flagged').
-- 'suspected' means the blob is in the 70-90% auto-classification range:
-- no label has been published yet; blob owners are prompted to confirm or ignore.
ALTER TABLE blob_moderation_state
    DROP CONSTRAINT IF EXISTS blob_moderation_state_harm_state_check;
ALTER TABLE blob_moderation_state
    ADD CONSTRAINT blob_moderation_state_harm_state_check
    CHECK (harm_state IN ('clean', 'suspected', 'blocked'));

-- Add index for efficient lookup of suspected blobs (for viewer state hydration).
CREATE INDEX IF NOT EXISTS idx_blob_moderation_state_suspected
    ON blob_moderation_state(blob_cid) WHERE harm_state = 'suspected';
