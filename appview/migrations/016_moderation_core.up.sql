-- Signed atproto label records (materialized per save URI).
-- One row per label issuance; negations are separate rows with neg=true.
CREATE TABLE label (
    id         BIGSERIAL PRIMARY KEY,
    src        TEXT        NOT NULL,                    -- labeler DID
    uri        TEXT        NOT NULL,                    -- at-uri of save record
    cid        TEXT,
    val        TEXT        NOT NULL,
    neg        BOOLEAN     NOT NULL DEFAULT FALSE,
    cts        TIMESTAMPTZ NOT NULL,
    exp        TIMESTAMPTZ,
    sig        BYTEA       NOT NULL,                    -- secp256k1 over CBOR canonical form
    ver        INT         NOT NULL DEFAULT 1,
    blob_cid   TEXT,                                    -- denormalized for blob-keyed hydration
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_label_uri_active      ON label(uri)      WHERE neg = FALSE;
CREATE INDEX idx_label_blob_cid_active ON label(blob_cid) WHERE neg = FALSE;

-- Per-blob moderation state, keyed by exact byte hash (the dedup unit).
CREATE TABLE blob_moderation_state (
    blob_cid      TEXT PRIMARY KEY,
    harm_state    TEXT        NOT NULL DEFAULT 'clean', -- clean | flagged | blocked
    ai_generated  BOOLEAN     NOT NULL DEFAULT FALSE,
    decided_by    TEXT,                                  -- 'auto' or reviewer DID
    decided_at    TIMESTAMPTZ,
    safety_scores JSONB,                                 -- {"nsfw":..,"violence":..,"ai_generated":..}
    notes         TEXT,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_blob_moderation_state_blocked
    ON blob_moderation_state(blob_cid) WHERE harm_state = 'blocked';

-- Append-only audit log for every moderation action (auto + human).
CREATE TABLE moderation_event (
    id          BIGSERIAL PRIMARY KEY,
    actor_did   TEXT        NOT NULL,                   -- 'auto' or moderator DID
    action      TEXT        NOT NULL,                   -- label_add | label_negate | takedown | acknowledge | ai_flag | self_label
    subject_uri TEXT,
    subject_cid TEXT,
    blob_cid    TEXT,
    payload     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_moderation_event_blob    ON moderation_event(blob_cid,    created_at);
CREATE INDEX idx_moderation_event_subject ON moderation_event(subject_uri, created_at);
