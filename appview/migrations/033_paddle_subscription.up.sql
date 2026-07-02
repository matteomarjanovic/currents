-- Mirror of Paddle subscription state (supporter tier), populated by the
-- /api/paddle/webhook handler. One row per Paddle subscription; a user (did)
-- is a supporter while any of their rows is in an access-granting status.
CREATE TABLE paddle_subscription (
    subscription_id TEXT PRIMARY KEY,
    did TEXT NOT NULL,
    customer_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    price_id TEXT NOT NULL DEFAULT '',
    scheduled_change TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX paddle_subscription_did_idx ON paddle_subscription (did);
