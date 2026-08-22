-- Per-user feed personalization preferences. The stored collection URIs are
-- excluded before source collections are sampled for either personalized mode.
CREATE TABLE feed_pref (
    viewer_did           TEXT        PRIMARY KEY,
    excluded_collections TEXT[]      NOT NULL DEFAULT '{}',
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
