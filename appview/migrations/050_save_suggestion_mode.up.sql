ALTER TABLE user_pref
    ADD COLUMN save_suggestion_mode TEXT NOT NULL DEFAULT 'recommended-then-last-used'
    CHECK (save_suggestion_mode IN ('last-used', 'recommended', 'recommended-then-last-used'));

CREATE INDEX idx_collection_author_with_embedding
    ON collection (author_did)
    WHERE canonical_embedding IS NOT NULL;
