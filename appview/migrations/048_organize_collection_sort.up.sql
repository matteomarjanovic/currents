CREATE TABLE pinned_collection (
    viewer_did     TEXT        NOT NULL,
    collection_uri TEXT        NOT NULL,
    pinned_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (viewer_did, collection_uri)
);

ALTER TABLE user_pref
    ADD COLUMN organize_collection_sort TEXT NOT NULL DEFAULT 'name'
    CHECK (organize_collection_sort IN ('name', 'recent'));
