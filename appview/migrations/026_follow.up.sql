CREATE TABLE follow (
    uri          TEXT        NOT NULL PRIMARY KEY,
    follower_did TEXT        NOT NULL,
    subject_did  TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (follower_did, subject_did)
);
CREATE INDEX follow_follower_idx ON follow (follower_did);
CREATE INDEX follow_subject_idx  ON follow (subject_did);
