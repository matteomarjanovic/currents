-- Keep buffered TAP records from recreating an account after cleanup.
CREATE TABLE tap_deleted_repo (
    did TEXT PRIMARY KEY,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
