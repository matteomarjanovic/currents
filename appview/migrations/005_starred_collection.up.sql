CREATE TABLE starred_collection (
    viewer_did     TEXT        NOT NULL,
    collection_uri TEXT        NOT NULL,
    starred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (viewer_did, collection_uri)
);
