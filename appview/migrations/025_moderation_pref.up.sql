-- Per-user moderation rendering preferences, server-backed so they follow the
-- user across browsers and devices (web + mobile). One row per user; absence of
-- a row means the user is on the defaults below.
CREATE TABLE moderation_pref (
    viewer_did    TEXT        PRIMARY KEY,
    porn          TEXT        NOT NULL DEFAULT 'blur',
    sexual        TEXT        NOT NULL DEFAULT 'blur',
    nudity        TEXT        NOT NULL DEFAULT 'blur',
    graphic_media TEXT        NOT NULL DEFAULT 'blur',
    ai_generated  TEXT        NOT NULL DEFAULT 'show',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT moderation_pref_adult_check CHECK (
        porn          IN ('show', 'blur', 'hide') AND
        sexual        IN ('show', 'blur', 'hide') AND
        nudity        IN ('show', 'blur', 'hide') AND
        graphic_media IN ('show', 'blur', 'hide')
    ),
    CONSTRAINT moderation_pref_ai_check CHECK (ai_generated IN ('show', 'hide'))
);
