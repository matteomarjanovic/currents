-- Record-backed favourites: replace the local-only starred_collection table
-- (never populated) with one keyed on the favourite record's own AT-URI, so TAP
-- delete events (which carry only the favourite record URI) can be applied.
-- Mirrors the follow table shape. The table is empty today, so the drop is safe.
DROP TABLE IF EXISTS starred_collection;

CREATE TABLE favourite_collection (
    uri            TEXT        NOT NULL PRIMARY KEY,
    viewer_did     TEXT        NOT NULL,
    collection_uri TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (viewer_did, collection_uri)
);
CREATE INDEX favourite_collection_viewer_idx     ON favourite_collection (viewer_did);
CREATE INDEX favourite_collection_collection_idx ON favourite_collection (collection_uri);
