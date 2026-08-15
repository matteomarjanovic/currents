"""Image decode/prepare regression tests (no model load, no server).

These guard the save-upload path: /prepare/image shrinks oversized uploads
under the PDS blob limit, and /embed/image /palette decode whatever the
browser sends.

Run with:

    ./venv/bin/python -m unittest test_image_prep -v
"""

import io
import unittest

import numpy as np
from fastapi import HTTPException
from PIL import Image

import main


def _png_bytes(image: Image.Image) -> bytes:
    out = io.BytesIO()
    image.save(out, format="PNG")
    return out.getvalue()


def _noise(width: int, height: int) -> Image.Image:
    rng = np.random.default_rng(7)
    return Image.fromarray(rng.integers(0, 256, size=(height, width, 3), dtype=np.uint8), "RGB")


class DeviceDtypeTests(unittest.TestCase):
    def test_mps_uses_bfloat16_and_cpu_uses_float32(self):
        self.assertEqual(main._dtype_for_device("mps"), main.torch.bfloat16)
        self.assertEqual(main._dtype_for_device("cpu"), main.torch.float32)


class DecodeImageTests(unittest.TestCase):
    def test_png_decodes_to_rgb_with_mime(self):
        image, mime = main._decode_image(_png_bytes(Image.new("RGB", (8, 8), (10, 20, 30))))
        self.assertEqual(mime, "image/png")
        self.assertEqual(image.mode, "RGB")
        self.assertEqual(image.size, (8, 8))

    def test_rgba_flattens_to_rgb(self):
        image, _ = main._decode_image(_png_bytes(Image.new("RGBA", (8, 8), (10, 20, 30, 128))))
        self.assertEqual(image.mode, "RGB")

    def test_jpeg_mime(self):
        out = io.BytesIO()
        Image.new("RGB", (8, 8)).save(out, format="JPEG")
        _, mime = main._decode_image(out.getvalue())
        self.assertEqual(mime, "image/jpeg")

    def test_garbage_raises_400(self):
        with self.assertRaises(HTTPException) as ctx:
            main._decode_image(b"not an image")
        self.assertEqual(ctx.exception.status_code, 400)


class PrepareImageBytesTests(unittest.TestCase):
    def test_shrinks_under_budget_as_jpeg(self):
        prepared = main._prepare_image_bytes(_noise(1024, 768), 60_000)
        self.assertLessEqual(len(prepared), 60_000)
        decoded = Image.open(io.BytesIO(prepared))
        self.assertEqual(decoded.format, "JPEG")
        self.assertLess(decoded.width, 1024)
        self.assertLess(decoded.height, 768)

    def test_impossible_budget_raises(self):
        # Even a 1×1 JPEG carries a few hundred bytes of headers, so a
        # 100-byte budget can never be met within PREPARE_MAX_STEPS.
        with self.assertRaises(ValueError):
            main._prepare_image_bytes(_noise(512, 512), 100)

    def test_non_positive_budget_raises(self):
        with self.assertRaises(ValueError):
            main._prepare_image_bytes(Image.new("RGB", (8, 8)), 0)


class L2NormalizeTests(unittest.TestCase):
    def test_rows_become_unit_norm(self):
        normed = main._l2_normalize(np.array([[3.0, 4.0], [0.5, 0.0]]))
        np.testing.assert_allclose(np.linalg.norm(normed, axis=1), [1.0, 1.0])

    def test_zero_rows_stay_finite(self):
        # A zero embedding must pass through unchanged, not become NaN —
        # safety classification runs on these rows.
        normed = main._l2_normalize(np.zeros((2, 4)))
        np.testing.assert_array_equal(normed, np.zeros((2, 4)))


if __name__ == "__main__":
    unittest.main()
