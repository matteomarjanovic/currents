CREATE TABLE global_feed_snapshot (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL
);

-- Derived ordering only. Live reads resolve each visual identity's current
-- canonical save and apply moderation and viewer filters before taking a page.
-- No visual-identity FK: deletion must not rewrite retained rankings.
CREATE TABLE global_feed_rank (
    snapshot_id BIGINT NOT NULL REFERENCES global_feed_snapshot(id) ON DELETE CASCADE,
    position BIGINT NOT NULL,
    visual_identity_id UUID NOT NULL,
    PRIMARY KEY (snapshot_id, position)
);
