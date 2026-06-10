CREATE TABLE seen_feature (
    viewer_did  TEXT        NOT NULL,
    feature_key TEXT        NOT NULL,
    seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (viewer_did, feature_key)
);
