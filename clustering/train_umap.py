"""
train_umap.py — Monthly UMAP model training.

Samples up to 100k embeddings from visual_identity, fits a UMAP model,
saves it atomically to MODELS_DIR, then re-projects all existing embeddings
in the DB so the entire table is on the new model's coordinate space.
Finally notifies the inference server to hot-reload the model.
"""
import os
import logging
import urllib.request

import joblib
import numpy as np
import psycopg2
from psycopg2.extras import execute_batch
from pgvector.psycopg2 import register_vector
from umap import UMAP

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

DATABASE_URL  = os.environ["DATABASE_URL"]
MODELS_DIR    = os.environ.get("MODELS_DIR", "./models")
UMAP_PATH     = os.path.join(MODELS_DIR, "umap_model.joblib")
INFERENCE_URL = os.environ.get("INFERENCE_URL", "")
TRAIN_LIMIT   = 100_000
BATCH_SIZE    = 2_000


def get_conn():
    conn = psycopg2.connect(DATABASE_URL)
    register_vector(conn)
    return conn


def train_umap():
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
    joblib.dump(reducer, tmp_path)
    os.replace(tmp_path, UMAP_PATH)
    log.info("UMAP model saved to %s", UMAP_PATH)

    _reproject_all(reducer)
    _notify_inference()


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
