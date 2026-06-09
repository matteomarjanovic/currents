-- Authorised moderators (admins + reviewers). Looked up by session DID.
CREATE TABLE moderator (
    did         TEXT PRIMARY KEY,
    role        TEXT        NOT NULL,                   -- admin | reviewer
    added_by    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    disabled_at TIMESTAMPTZ
);

-- User-submitted reports (via com.atproto.moderation.createReport).
CREATE TABLE report (
    id           BIGSERIAL PRIMARY KEY,
    reporter_did TEXT        NOT NULL,
    subject_uri  TEXT        NOT NULL,
    subject_cid  TEXT,
    reason_type  TEXT        NOT NULL,                  -- com.atproto.moderation.defs#reason*
    reason_text  TEXT,
    status       TEXT        NOT NULL DEFAULT 'pending',-- pending | resolved | dismissed
    resolved_by  TEXT,
    resolved_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_report_pending ON report(created_at)  WHERE status = 'pending';
CREATE INDEX idx_report_subject ON report(subject_uri);

-- Pending items in the review queue (auto-flags, reports, manual additions).
-- AI-generated never enters this queue (informational axis).
CREATE TABLE review_item (
    id          BIGSERIAL PRIMARY KEY,
    source      TEXT        NOT NULL,                   -- ai | report | self_label | manual
    source_ref  BIGINT,                                  -- FK-like reference (report.id) when source=report
    subject_uri TEXT        NOT NULL,
    subject_cid TEXT,
    blob_cid    TEXT,
    category    TEXT,                                    -- nsfw | violence | other
    score       REAL,                                    -- null for non-AI sources
    status      TEXT        NOT NULL DEFAULT 'pending', -- pending | resolved | dismissed
    priority    INT         NOT NULL DEFAULT 0,         -- 0 normal, 100 high (auto-suspected)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_review_item_queue ON review_item(status, priority DESC, created_at);
CREATE INDEX idx_review_item_blob  ON review_item(blob_cid);
