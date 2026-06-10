DROP INDEX IF EXISTS idx_blob_moderation_state_suspected;

ALTER TABLE blob_moderation_state
    DROP CONSTRAINT IF EXISTS blob_moderation_state_harm_state_check;
