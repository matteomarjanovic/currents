CREATE TABLE import_session (
    id         UUID PRIMARY KEY,
    owner_did  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_import_session_owner ON import_session(owner_did);

CREATE TABLE import_job (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id            UUID NOT NULL REFERENCES import_session(id) ON DELETE CASCADE,
    owner_did             TEXT NOT NULL,
    oauth_session_id      TEXT NOT NULL,
    source                TEXT NOT NULL,
    source_board_id       TEXT NOT NULL,
    source_board_name     TEXT NOT NULL DEFAULT '',
    target_collection_uri TEXT NOT NULL,
    status                TEXT NOT NULL DEFAULT 'listing',
    list_cursor           TEXT NOT NULL DEFAULT '',
    error                 TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_import_job_session ON import_job(session_id);
CREATE INDEX idx_import_job_owner_status ON import_job(owner_did, status);

CREATE TABLE import_item (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID NOT NULL REFERENCES import_job(id) ON DELETE CASCADE,
    owner_did     TEXT NOT NULL,
    source_pin_id TEXT NOT NULL,
    image_url     TEXT NOT NULL,
    rkey          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'queued',
    save_uri      TEXT NOT NULL DEFAULT '',
    error         TEXT NOT NULL DEFAULT '',
    attempt_count INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, source_pin_id)
);

CREATE INDEX idx_import_item_owner_status ON import_item(owner_did, status);
CREATE INDEX idx_import_item_job_status   ON import_item(job_id, status);
