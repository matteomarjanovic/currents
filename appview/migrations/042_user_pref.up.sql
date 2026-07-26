-- Per-user general preferences, server-backed so they follow the user across
-- browsers and devices (web + mobile). One row per user; absence of a row means
-- the user is on the defaults below. Distinct from moderation_pref, which gates
-- content visibility.
CREATE TABLE user_pref (
    viewer_did   TEXT        PRIMARY KEY,
    gif_autoplay BOOLEAN     NOT NULL DEFAULT true,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
