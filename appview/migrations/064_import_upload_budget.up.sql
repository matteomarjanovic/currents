CREATE TABLE import_upload_budget (
    owner_did TEXT NOT NULL,
    pds_url TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY (owner_did, pds_url)
);
