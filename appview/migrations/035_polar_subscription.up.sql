-- Supporter tier moves from Paddle to Polar. Same design: a mirror of
-- subscription state populated by the /api/polar/webhook handler, one row per
-- Polar subscription; a user (did) is a supporter while any of their rows is
-- in an access-granting status. Paddle never launched (no production rows),
-- so the old mirror is dropped rather than migrated.
DROP TABLE paddle_subscription;

CREATE TABLE polar_subscription (
    subscription_id TEXT PRIMARY KEY,
    did TEXT NOT NULL,
    customer_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    product_id TEXT NOT NULL DEFAULT '',
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX polar_subscription_did_idx ON polar_subscription (did);
