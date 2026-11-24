CREATE TABLE external_label (
    src TEXT NOT NULL,
    uri TEXT NOT NULL,
    cid TEXT NOT NULL,
    val TEXT NOT NULL,
    cts TIMESTAMPTZ NOT NULL,
    exp TIMESTAMPTZ,
    neg BOOLEAN NOT NULL,
    PRIMARY KEY (src, uri, cid, val)
);

CREATE TABLE external_label_cursor (
    src TEXT PRIMARY KEY,
    seq BIGINT NOT NULL
);
