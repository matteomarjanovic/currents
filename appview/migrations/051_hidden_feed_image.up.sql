CREATE TABLE hidden_feed_image (
    viewer_did         TEXT        NOT NULL,
    visual_identity_id UUID        NOT NULL REFERENCES visual_identity(id) ON DELETE CASCADE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (viewer_did, visual_identity_id)
);
