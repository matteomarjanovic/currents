import json
import logging
import os
from datetime import datetime

import psycopg2

log = logging.getLogger(__name__)


def record_job_run(job: str, status: str, started_at: datetime, details: dict):
    """Best-effort operational telemetry must never fail the scheduled job."""
    try:
        conn = psycopg2.connect(os.environ["DATABASE_URL"])
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO operations_job_run (job, status, started_at, finished_at, details)
                    VALUES (%s, %s, %s, now(), %s::jsonb)
                    """,
                    (job, status, started_at, json.dumps(details)),
                )
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("Could not record %s job run: %s", job, e)
