"""Feed-junk scoring regression tests (no model load, no server).

Run with:

    ./venv/bin/python -m unittest test_junk_head -v
"""

import math
import unittest

import numpy as np

import main


class _FakeSession:
    """Stands in for the ONNX session: returns fixed logits for a batch."""

    def __init__(self, logits):
        self._logits = np.asarray(logits, dtype=np.float32).reshape(-1, 1)

    def run(self, _outputs, feeds):
        n = feeds["embedding"].shape[0]
        return [self._logits[:n]]


class ScoreJunkTests(unittest.TestCase):
    def tearDown(self):
        main._junk_head = None

    def test_none_without_head(self):
        self.assertIsNone(main._score_junk(np.zeros((2, 768), np.float32)))

    def test_sigmoid_of_logits(self):
        main._junk_head = _FakeSession([0.0, 2.0, -2.0])
        scores = main._score_junk(np.zeros((3, 768), np.float32))
        self.assertEqual(len(scores), 3)
        self.assertAlmostEqual(scores[0], 0.5, places=6)
        self.assertAlmostEqual(scores[1], 1 / (1 + math.exp(-2)), places=6)
        self.assertAlmostEqual(scores[2], 1 / (1 + math.exp(2)), places=6)
        for s in scores:
            self.assertGreaterEqual(s, 0.0)
            self.assertLessEqual(s, 1.0)

    def test_extreme_logits_stay_finite(self):
        # sigmoid runs in float64 so ±1000 logits must not overflow to NaN
        main._junk_head = _FakeSession([1000.0, -1000.0])
        scores = main._score_junk(np.zeros((2, 768), np.float32))
        self.assertEqual(scores, [1.0, 0.0])


if __name__ == "__main__":
    unittest.main()
