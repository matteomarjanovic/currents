-- Per-user "last seen" marker for the Activity (social) notifications tab.
-- Server-backed so the unread state follows the user across devices, like
-- seen_feature and moderation_pref. Absence of a row means "never seen" — every
-- follower counts as unseen (reads COALESCE to epoch).
CREATE TABLE notification_seen (
    viewer_did     TEXT        NOT NULL PRIMARY KEY,
    social_seen_at TIMESTAMPTZ NOT NULL
);
