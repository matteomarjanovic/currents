CREATE TABLE admin (
    did         TEXT PRIMARY KEY,
    added_by    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    disabled_at TIMESTAMPTZ
);

-- Preserve access for the people who were already moderation administrators,
-- while keeping the two grants independent from this point onward.
INSERT INTO admin (did, added_by, created_at)
SELECT did, added_by, created_at
FROM moderator
WHERE role = 'admin' AND disabled_at IS NULL
ON CONFLICT (did) DO NOTHING;

CREATE TABLE operations_job_run (
    id          BIGSERIAL PRIMARY KEY,
    job         TEXT        NOT NULL CHECK (job IN ('postgres_backup', 'umap_train', 'clustering')),
    status      TEXT        NOT NULL CHECK (status IN ('success', 'failed')),
    started_at  TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    details     JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_operations_job_run_latest ON operations_job_run(job, finished_at DESC);

CREATE TABLE operations_host_snapshot (
    host        TEXT        PRIMARY KEY CHECK (host IN ('main', 'inference')),
    reported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload     JSONB       NOT NULL
);
