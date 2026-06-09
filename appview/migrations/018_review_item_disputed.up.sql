-- Self-attestation: track when an author has disputed an auto-flag on their own
-- save. Dispute does NOT clear the suspected label (that's a moderator-only
-- decision); it raises the moderator-queue priority and flags the item.
ALTER TABLE review_item
    ADD COLUMN disputed    BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN disputed_at TIMESTAMPTZ;

CREATE INDEX idx_review_item_disputed_pending
    ON review_item(priority DESC, created_at)
    WHERE disputed = TRUE AND status = 'pending';
