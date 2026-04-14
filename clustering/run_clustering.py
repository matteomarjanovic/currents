"""
run_clustering.py — Daily HDBSCAN clustering.

Backfills any missing umap_embeddings, then clusters all VIs in UMAP space,
and atomically swaps in the new cluster assignments.
"""
import os
import logging
from datetime import date

import hdbscan
import joblib
import numpy as np
import psycopg2
from psycopg2.extras import execute_batch
from pgvector.psycopg2 import register_vector

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

DATABASE_URL = os.environ["DATABASE_URL"]
MODELS_DIR   = os.environ.get("MODELS_DIR", "./models")
UMAP_PATH    = os.path.join(MODELS_DIR, "umap_model.joblib")
MIN_POINTS   = 100   # skip clustering if fewer points than this


def get_conn():
    conn = psycopg2.connect(DATABASE_URL)
    register_vector(conn)
    return conn


def run_clustering():
    if not os.path.exists(UMAP_PATH):
        log.warning("No UMAP model at %s — skipping clustering (run train_umap.py first)", UMAP_PATH)
        return

    reducer = joblib.load(UMAP_PATH)
    log.info("Loaded UMAP model from %s", UMAP_PATH)

    _backfill_umap(reducer)

    log.info("Loading all UMAP embeddings for clustering")
    conn = get_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT id, umap_embedding
                FROM visual_identity
                WHERE umap_embedding IS NOT NULL
            """)
            rows = cur.fetchall()
    finally:
        conn.close()

    if len(rows) < MIN_POINTS:
        log.info("Only %d points available — skipping clustering (need %d)", len(rows), MIN_POINTS)
        return

    vi_ids = [r[0] for r in rows]
    X = np.array([r[1] for r in rows], dtype=np.float32)
    log.info("Clustering %d points in 50-dim UMAP space", len(vi_ids))

    clusterer = hdbscan.HDBSCAN(
        min_cluster_size=10,
        min_samples=5,
        metric="euclidean",
        core_dist_n_jobs=-1,
        approx_min_span_tree=True,
    )
    labels = clusterer.fit_predict(X)

    unique_labels = set(labels) - {-1}
    noise_count   = int(np.sum(labels == -1))
    log.info("Found %d clusters, %d noise points", len(unique_labels), noise_count)

    if not unique_labels:
        log.warning("No clusters found — skipping DB write")
        return

    _write_clusters(vi_ids, X, labels, unique_labels)


def _backfill_umap(reducer):
    """Apply UMAP transform to any VIs that have a 768-dim embedding but no umap_embedding."""
    conn = get_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT id, embedding
                FROM visual_identity
                WHERE embedding IS NOT NULL AND umap_embedding IS NULL
            """)
            rows = cur.fetchall()

        if not rows:
            return

        log.info("Backfilling %d missing UMAP embeddings", len(rows))
        ids  = [r[0] for r in rows]
        embs = np.array([r[1] for r in rows], dtype=np.float32)
        reduced = reducer.transform(embs)

        with conn.cursor() as cur:
            execute_batch(cur, """
                UPDATE visual_identity SET umap_embedding = %s WHERE id = %s
            """, [(reduced[i].tolist(), ids[i]) for i in range(len(ids))], page_size=500)
        conn.commit()
        log.info("Backfill complete")
    finally:
        conn.close()


def _find_medoid(X: np.ndarray) -> int:
    """Return the index of the medoid (min total L2 distance to all other points).
    Approximated using a random 500-point sample for large clusters."""
    if len(X) == 1:
        return 0
    sample = X
    if len(X) > 500:
        idx    = np.random.choice(len(X), 500, replace=False)
        sample = X[idx]
    dists = np.sum(np.linalg.norm(X[:, None] - sample[None, :], axis=2), axis=1)
    return int(np.argmin(dists))


def _write_clusters(vi_ids, X, labels, unique_labels):
    """Atomically insert new clusters, update VI assignments, and delete old clusters."""
    today = date.today()
    conn  = get_conn()
    try:
        # Build cluster data before touching the DB.
        # cluster_data: list of (medoid_vi_id, size, list_of_vi_ids)
        cluster_data = []
        for lbl in unique_labels:
            mask          = labels == lbl
            cluster_vi    = [vi_ids[i] for i in range(len(vi_ids)) if mask[i]]
            cluster_X     = X[mask]
            medoid_idx    = _find_medoid(cluster_X)
            medoid_vi_id  = cluster_vi[medoid_idx]
            cluster_data.append((str(medoid_vi_id), int(np.sum(mask)), cluster_vi))

        with conn.cursor() as cur:
            # 1. Insert new cluster rows. medoid FK references existing VI rows — always valid.
            cluster_uuid_to_vi_ids = {}
            for medoid_vi_id, size, vi_id_list in cluster_data:
                cur.execute("""
                    INSERT INTO cluster (run_date, size, medoid_visual_identity_id)
                    VALUES (%s, %s, %s)
                    RETURNING id
                """, (today, size, medoid_vi_id))
                cluster_uuid = str(cur.fetchone()[0])
                cluster_uuid_to_vi_ids[cluster_uuid] = vi_id_list

            # 2. Update VI cluster_id assignments. New cluster rows exist so FK is valid.
            updates = [
                (cluster_uuid, str(vi_id))
                for cluster_uuid, vi_id_list in cluster_uuid_to_vi_ids.items()
                for vi_id in vi_id_list
            ]
            execute_batch(cur, """
                UPDATE visual_identity SET cluster_id = %s WHERE id = %s
            """, updates, page_size=1000)

            # 3. Delete old cluster rows. No VIs reference them anymore (step 2 replaced all).
            cur.execute("DELETE FROM cluster WHERE run_date < %s", (today,))
            deleted = cur.rowcount
            log.info("Deleted %d stale cluster rows", deleted)

        conn.commit()
        log.info(
            "Cluster swap complete: %d clusters written for %s",
            len(cluster_data), today,
        )
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()


if __name__ == "__main__":
    run_clustering()
