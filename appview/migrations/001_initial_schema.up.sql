CREATE TABLE IF NOT EXISTS oauth_sessions (
	account_did  TEXT        NOT NULL,
	session_id   TEXT        NOT NULL,
	data         JSONB       NOT NULL,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (account_did, session_id)
);

CREATE TABLE IF NOT EXISTS oauth_auth_requests (
	state      TEXT        NOT NULL PRIMARY KEY,
	data       JSONB       NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS "user" (
	did           TEXT PRIMARY KEY,
	handle        TEXT,
	display_name  TEXT,
	description   TEXT,
	pronouns      TEXT,
	website       TEXT,
	avatar        TEXT,
	banner        TEXT,
	created_at    TIMESTAMPTZ NOT NULL,
	pds_endpoint  TEXT
);

CREATE TABLE IF NOT EXISTS collection (
	uri         TEXT PRIMARY KEY,
	author_did  TEXT NOT NULL,
	name        TEXT NOT NULL,
	description TEXT,
	created_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS save (
	uri            TEXT PRIMARY KEY,
	author_did     TEXT NOT NULL,
	collection_uri TEXT NOT NULL,
	pds_blob_cid   TEXT NOT NULL,
	text           TEXT,
	origin_url     TEXT,
	resave_of_uri  TEXT,
	created_at     TIMESTAMPTZ
);
