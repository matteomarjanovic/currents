-- Pending background deletions of all is.currents.* records from a user's PDS
-- after account deletion. Record deletes are rate-limited by the PDS (5,000
-- write points/hour per account, DELETE = 1 point), so large repos can't be
-- wiped in-request. The referenced oauth session is kept alive by
-- DeleteUserData; the wipe worker re-enumerates the repo on each pass, so no
-- per-record progress state is stored.
CREATE TABLE pds_wipe (
    did TEXT PRIMARY KEY,
    oauth_session_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
