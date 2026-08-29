"""
train_umap.py — Monthly UMAP model training.

Samples up to 100k embeddings from visual_identity, fits a UMAP model,
saves it atomically to MODELS_DIR, and publishes it to the model store
(Scaleway Object Storage) for the inference server, which runs on other
hardware and syncs the bucket on a cron (see SCALEWAY_MIGRATION.md). Then
re-projects all existing embeddings in the DB so the entire table is on the
new model's coordinate space. Finally notifies the inference server to
hot-reload the model — a no-op unless INFERENCE_URL is set (dev, where
inference shares the models directory and needs no bucket round-trip).
"""
import os
import logging
import urllib.request
from datetime import datetime, timezone

import joblib
import numpy as np
import psycopg2
from psycopg2.extras import execute_batch
from pgvector.psycopg2 import register_vector
from umap import UMAP

from operations import record_job_run

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

DATABASE_URL  = os.environ["DATABASE_URL"]
MODELS_DIR    = os.environ.get("MODELS_DIR", "./models")
UMAP_PATH     = os.path.join(MODELS_DIR, "umap_model.joblib")
INFERENCE_URL = os.environ.get("INFERENCE_URL", "")
S3_ENDPOINT   = os.environ.get("MODELS_S3_ENDPOINT", "")
S3_BUCKET     = os.environ.get("MODELS_S3_BUCKET", "")
TRAIN_LIMIT   = 100_000
BATCH_SIZE    = 2_000


def get_conn():
    conn = psycopg2.connect(DATABASE_URL)
    register_vector(conn)
    return conn


def train_umap():
    started_at = datetime.now(timezone.utc)
    try:
        log.info("Sampling embeddings for UMAP training (limit=%d)", TRAIN_LIMIT)
        conn = get_conn()
        try:
            with conn.cursor() as cur:
                cur.execute("""
                    SELECT embedding
                    FROM visual_identity
                    WHERE embedding IS NOT NULL
                    ORDER BY random()
                    LIMIT %s
                """, (TRAIN_LIMIT,))
                rows = cur.fetchall()
        finally:
            conn.close()

        if not rows:
            log.warning("No embeddings found — skipping UMAP training")
            record_job_run("umap_train", "success", started_at, {"sampled": 0, "skipped": True})
            return

        X = np.array([r[0] for r in rows], dtype=np.float32)
        log.info("Training UMAP on %d embeddings (input_dim=%d)", len(X), X.shape[1])

        reducer = UMAP(
            n_components=50,
            n_neighbors=15,
            min_dist=0.1,
            metric="cosine",
            random_state=42,
            low_memory=True,
        )
        reducer.fit(X)
        log.info("UMAP training complete")

        os.makedirs(MODELS_DIR, exist_ok=True)
        tmp_path = UMAP_PATH + ".tmp"
        # compress: the pynndescent index pickles to ~1 GB uncompressed; zlib-3
        # shrinks it several-fold and joblib.load handles it transparently.
        joblib.dump(reducer, tmp_path, compress=3)
        os.replace(tmp_path, UMAP_PATH)
        log.info("UMAP model saved to %s", UMAP_PATH)

        _publish_model()
        reprojected = _reproject_all(reducer)
        _notify_inference()
    except Exception as e:
        record_job_run("umap_train", "failed", started_at, {"error": str(e)[:1000]})
        raise
    record_job_run("umap_train", "success", started_at, {
        "sampled": len(rows),
        "reprojected": reprojected,
        "modelBytes": os.path.getsize(UMAP_PATH),
    })


def _reproject_all(reducer: UMAP):
    """Re-project every visual_identity embedding with the new model, in streaming batches."""
    log.info("Re-projecting all existing embeddings (batch_size=%d)", BATCH_SIZE)
    conn = get_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT COUNT(*) FROM visual_identity WHERE embedding IS NOT NULL")
            total = cur.fetchone()[0]
        log.info("Total rows to re-project: %d", total)

        offset = 0
        updated = 0
        while True:
            with conn.cursor() as cur:
                cur.execute("""
                    SELECT id, embedding
                    FROM visual_identity
                    WHERE embedding IS NOT NULL
                    ORDER BY id
                    LIMIT %s OFFSET %s
                """, (BATCH_SIZE, offset))
                rows = cur.fetchall()

            if not rows:
                break

            ids  = [r[0] for r in rows]
            embs = np.array([r[1] for r in rows], dtype=np.float32)
            reduced = reducer.transform(embs)

            with conn.cursor() as cur:
                execute_batch(cur, """
                    UPDATE visual_identity SET umap_embedding = %s WHERE id = %s
                """, [(reduced[i].tolist(), ids[i]) for i in range(len(ids))], page_size=500)
            conn.commit()

            updated += len(rows)
            offset  += BATCH_SIZE
            log.info("Re-projected %d / %d", updated, total)
    finally:
        conn.close()

    log.info("Re-projection complete: %d rows updated", updated)
    return updated


def _publish_model():
    """Upload the saved model to the model store. No-op when unconfigured
    (dev). Deliberately not wrapped in try/except: a failed upload must fail
    the cron run loudly, because the inference server would otherwise keep
    syncing a stale model with no signal."""
    if not S3_BUCKET:
        return
    import boto3  # deferred: not needed on the no-op path

    s3 = boto3.client("s3", endpoint_url=S3_ENDPOINT or None)
    s3.upload_file(UMAP_PATH, S3_BUCKET, os.path.basename(UMAP_PATH))
    log.info("UMAP model published to s3://%s/%s", S3_BUCKET, os.path.basename(UMAP_PATH))


def _notify_inference():
    if not INFERENCE_URL:
        return
    try:
        urllib.request.urlopen(f"{INFERENCE_URL}/reload-umap", data=b"", timeout=5)
        log.info("Notified inference server to reload UMAP model")
    except Exception as e:
        log.warning("Could not notify inference server: %s", e)


if __name__ == "__main__":
    train_umap()
