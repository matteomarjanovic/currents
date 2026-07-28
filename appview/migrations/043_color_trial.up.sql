-- Free-trial ledger for color search. Non-supporters may sample the feature on
-- a lifetime allowance of distinct query colors (colorTrialLimit in appview);
-- one row per color spent. Keying on the color rather than the request is what
-- keeps pagination, hybrid-text refinement and scope changes off the meter —
-- they all reuse a color that was already paid for.
CREATE TABLE color_trial (
    viewer_did TEXT        NOT NULL,
    color_hex  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (viewer_did, color_hex)
);
