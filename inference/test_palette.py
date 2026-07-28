"""Palette-extraction regression tests (no model load, no server).

Run with:

    ./venv/bin/python -m unittest test_palette -v
"""

import io
import unittest

import numpy as np
from fastapi import HTTPException
from PIL import Image

import main


def _channels(hex_color: str) -> tuple[int, int, int]:
    return (
        int(hex_color[1:3], 16),
        int(hex_color[3:5], 16),
        int(hex_color[5:7], 16),
    )


class SRGBToLabTests(unittest.TestCase):
    def test_reference_values(self):
        # Textbook D65 values; the Go side (hexToLab in appview/visual.go) is
        # pinned to the same numbers, keeping both sides of the ΔE comparison
        # in one color space.
        lab = main._srgb_to_lab(np.array([
            [255.0, 255.0, 255.0],
            [0.0, 0.0, 0.0],
            [119.0, 119.0, 119.0],
            [255.0, 0.0, 0.0],
        ]))
        expected = np.array([
            [100.0, 0.0, 0.0],
            [0.0, 0.0, 0.0],
            [50.03, 0.0, 0.0],
            [53.24, 80.09, 67.20],
        ])
        np.testing.assert_allclose(lab, expected, atol=0.05)


class DominantColorsTests(unittest.TestCase):
    def setUp(self):
        # k-means seeds its centroids from an unseeded RNG; pin it so the
        # extracted palettes are reproducible.
        self._default_rng = np.random.default_rng
        np.random.default_rng = lambda *args: self._default_rng(*(args or (42,)))

    def tearDown(self):
        np.random.default_rng = self._default_rng

    def _assert_contract(self, colors):
        self.assertLessEqual(len(colors), main.PALETTE_SIZE)
        self.assertGreater(len(colors), 0)
        fractions = [c["fraction"] for c in colors]
        self.assertEqual(fractions, sorted(fractions, reverse=True))
        self.assertLessEqual(sum(fractions), 1.0001)
        for c in colors:
            self.assertEqual(set(c), {"hex", "fraction"})
            self.assertRegex(c["hex"], r"^#[0-9a-f]{6}$")
            self.assertGreater(c["fraction"], 0.0)

    def test_small_salient_accent_survives(self):
        # A ~1% saturated yellow patch on a muted beige/gray scene must appear
        # in the palette — the failure mode of the old area-averaging k-means.
        rng = np.random.default_rng(42)
        arr = np.zeros((256, 256, 3), np.uint8)
        arr[:, :] = (185, 172, 150)
        arr[128:, :] = (120, 118, 110)
        arr = np.clip(arr.astype(int) + rng.integers(-12, 12, arr.shape), 0, 255).astype(np.uint8)
        arr[60:86, 100:126] = (230, 190, 20)

        colors = main._dominant_colors(Image.fromarray(arr))
        self._assert_contract(colors)

        def yellowish(c):
            r, g, b = _channels(c["hex"])
            return r > 180 and g > 140 and b < 90

        accents = [c for c in colors if yellowish(c)]
        self.assertEqual(len(accents), 1, f"yellow accent missing from {colors}")
        self.assertAlmostEqual(accents[0]["fraction"], 0.01, delta=0.005)

    def test_solid_color_single_entry(self):
        colors = main._dominant_colors(Image.new("RGB", (256, 256), (20, 130, 140)))
        self._assert_contract(colors)
        self.assertEqual(colors, [{"hex": "#14828c", "fraction": 1.0}])

    def test_close_shades_merge(self):
        # Two grays within the merge threshold collapse to one palette entry.
        arr = np.zeros((128, 128, 3), np.uint8)
        arr[:64] = (128, 128, 128)
        arr[64:] = (131, 131, 131)
        colors = main._dominant_colors(Image.fromarray(arr))
        self.assertEqual(len(colors), 1)
        self.assertAlmostEqual(colors[0]["fraction"], 1.0, delta=0.001)

    def test_noise_floor_drops_single_pixel(self):
        arr = np.full((128, 128, 3), 245, np.uint8)
        arr[5, 5] = (220, 30, 30)
        colors = main._dominant_colors(Image.fromarray(arr))
        self._assert_contract(colors)
        for c in colors:
            r, g, b = _channels(c["hex"])
            self.assertFalse(r > 150 and g < 100, f"sub-floor speck surfaced: {colors}")

    def test_grayscale_stays_neutral(self):
        g = np.tile(np.linspace(0, 255, 256, dtype=np.uint8), (256, 1))
        colors = main._dominant_colors(Image.fromarray(np.stack([g, g, g], axis=-1)))
        self._assert_contract(colors)
        hexes = [c["hex"] for c in colors]
        self.assertEqual(len(hexes), len(set(hexes)))
        for c in colors:
            r, gr, b = _channels(c["hex"])
            self.assertLessEqual(max(r, gr, b) - min(r, gr, b), 2, f"non-neutral gray: {c}")


class PaletteFromBytesTests(unittest.TestCase):
    """The /palette entry point runs decode + extraction in one executor hop;
    it must stay byte-identical to the live indexing path, which extracts from
    the same full-resolution decode."""

    def _jpeg(self) -> bytes:
        arr = np.zeros((64, 64, 3), dtype=np.uint8)
        arr[:, :32] = (200, 40, 40)
        arr[:, 32:] = (40, 60, 200)
        buf = io.BytesIO()
        Image.fromarray(arr).save(buf, format="JPEG", quality=95)
        return buf.getvalue()

    def test_matches_decode_then_extract(self):
        raw = self._jpeg()
        expected = main._dominant_colors(main._decode_image(raw)[0])
        self.assertEqual(main._palette_from_bytes(raw), expected)

    def test_garbage_raises_400(self):
        with self.assertRaises(HTTPException) as ctx:
            main._palette_from_bytes(b"not an image")
        self.assertEqual(ctx.exception.status_code, 400)


if __name__ == "__main__":
    unittest.main()
