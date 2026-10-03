CREATE TABLE jetstream_cursor (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    seq BIGINT NOT NULL DEFAULT 0
);
INSERT INTO jetstream_cursor (singleton, seq) VALUES (TRUE, 0);

CREATE TABLE jetstream_repo (
    did TEXT PRIMARY KEY,
    state TEXT NOT NULL CHECK (state IN ('active', 'inactive', 'opted_out')),
    reconciled_rev TEXT NOT NULL DEFAULT ''
);
INSERT INTO jetstream_repo (did, state)
SELECT did, 'active' FROM "user";

CREATE TABLE jetstream_backfill (
    did TEXT PRIMARY KEY,
    due_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts INTEGER NOT NULL DEFAULT 0
);
