"""Medoid-selection regression tests. The clustering deps (hdbscan, psycopg2)
only exist in the service image, so run these in-container:

    docker compose -f docker-compose.dev.yml run --rm --build clustering python -m unittest -v
"""

import os
import unittest

os.environ.setdefault("DATABASE_URL", "postgres://unused")  # read at import time

import numpy as np

import run_clustering


class FindMedoidTests(unittest.TestCase):
    def test_single_point(self):
        self.assertEqual(run_clustering._find_medoid(np.zeros((1, 3), np.float32)), 0)

    def test_central_point_wins(self):
        # Total distance from 1.0 is smallest: 1 + 9 < 1 + 10 < 10 + 9.
        X = np.array([[0.0, 0.0], [1.0, 0.0], [10.0, 0.0]], np.float32)
        self.assertEqual(run_clustering._find_medoid(X), 1)

    def test_outlier_never_medoid(self):
        rng = np.random.default_rng(3)
        X = rng.normal(0, 1, (50, 8)).astype(np.float32)
        X[7] = 100.0
        self.assertNotEqual(run_clustering._find_medoid(X), 7)

    def test_sampled_path_stays_central(self):
        # >500 points triggers the random 500-point sample; the pick must
        # still be among the most central points of a Gaussian blob.
        np.random.seed(11)  # _find_medoid samples via the legacy np.random API
        rng = np.random.default_rng(5)
        X = rng.normal(0, 1, (600, 8)).astype(np.float32)
        idx = run_clustering._find_medoid(X)
        norms = np.linalg.norm(X, axis=1)
        self.assertLess(norms[idx], np.percentile(norms, 5))


if __name__ == "__main__":
    unittest.main()
