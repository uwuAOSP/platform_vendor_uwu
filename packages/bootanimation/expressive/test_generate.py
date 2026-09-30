# SPDX-FileCopyrightText: The uwuAOSP Project
# SPDX-License-Identifier: Apache-2.0
"""Host-side verification of geometry, timing, PNG masks, and AOSP package format."""

import io
import math
import unittest
import zipfile

from PIL import Image

import generate as animation


class BootAnimationTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.shapes = animation.load_shapes()

    def test_matches_google_motion_parameters(self):
        _, rotation_a, _ = animation.motion_at(0.5, self.shapes)
        _, rotation_b, _ = animation.motion_at(0.6, self.shapes)
        self.assertGreater(rotation_b - rotation_a, math.radians(3))
        progress, _ = animation.solve_spring(0, 0, 0.3)
        self.assertGreater(progress, 1.05)
        self.assertEqual(animation.FRAMES_PER_SEGMENT / animation.FPS, 0.650)

    def test_three_cycles_close_geometry_rotation_and_color(self):
        start = animation.INTRO_FRAMES / animation.FPS
        end = start + animation.LOOP_FRAMES / animation.FPS
        a = animation.motion_at(start, self.shapes, periodic=True)
        b = animation.motion_at(end, self.shapes, periodic=True)
        for x, y in zip(a[0], b[0]):
            self.assertAlmostEqual(x, y, places=8)
        self.assertAlmostEqual(b[1] - a[1], 7 * math.tau, places=8)
        for x, y in zip(a[2], b[2]):
            self.assertAlmostEqual(x, y, places=8)

    def test_startup_joins_steady_loop(self):
        t = animation.INTRO_FRAMES / animation.FPS
        a = animation.motion_at(t - 0.000001, self.shapes)
        b = animation.motion_at(t, self.shapes, periodic=True)
        self.assertLess(max(abs(x - y) for x, y in zip(a[0], b[0])), 0.001)
        self.assertLess(abs(a[1] - b[1]), 0.001)

    def test_all_package_entries_and_decoded_masks(self):
        for resolution, size in ((720, 128), (1080, 192), (1440, 256)):
            with self.subTest(resolution=resolution):
                path = animation.ROOT / f"bootanimation_{resolution}.zip"
                with zipfile.ZipFile(path) as archive:
                    self.assertIsNone(archive.testzip())
                    self.assertTrue(all(i.compress_type == zipfile.ZIP_STORED
                                        for i in archive.infolist()))
                    desc = archive.read("desc.txt").decode().splitlines()
                    self.assertEqual(desc[0], f"{resolution} {resolution} 60")
                    self.assertTrue(desc[1].startswith("dynamic_colors intro "))
                    self.assertEqual(desc[2], "p 1 0 intro #000000")
                    self.assertEqual(desc[3], "f 0 0 loop 12 #000000")
                    for part, count in (("intro", animation.INTRO_FRAMES),
                                        ("loop", animation.LOOP_FRAMES)):
                        frames = [name for name in archive.namelist()
                                  if name.startswith(part + "/") and name.endswith(".png")]
                        self.assertEqual(frames, [f"{part}/{n:05d}.png" for n in range(count)])
                        offset = (resolution - size) // 2
                        trim = archive.read(f"{part}/trim.txt").decode().splitlines()
                        self.assertEqual(trim, [f"{size}x{size}+{offset}+{offset}"] * count)
                        # Decode every frame: corrupt/channel-premultiplied PNGs cannot hide.
                        for name in frames:
                            image = Image.open(io.BytesIO(archive.read(name)))
                            self.assertEqual(image.mode, "RGBA")
                            self.assertEqual(image.size, (size, size))
                            extrema = image.getextrema()
                            self.assertGreaterEqual(max(hi for _, hi in extrema), 127)
                            self.assertLessEqual(sum(hi > 0 for _, hi in extrema), 2)
                            self.assertEqual(image.getpixel((0, 0)), (0, 0, 0, 0))
                            center = image.getpixel((size // 2, size // 2))
                            self.assertLessEqual(abs(sum(center) - 255), 1)
                    red_mask = Image.open(io.BytesIO(archive.read("intro/00000.png")))
                    self.assertEqual(red_mask.getpixel((size // 2, size // 2)), (255, 0, 0, 0))

    def test_palette_channels_cover_all_shapes(self):
        self.assertEqual(len(set(animation.CHANNELS)), 4)
        for i in range(6):
            _, _, weights = animation.motion_at(i * 0.650 + 0.001, self.shapes, periodic=True)
            self.assertAlmostEqual(sum(weights), 1)
            self.assertTrue(all(0 <= w <= 1 for w in weights))


if __name__ == "__main__":
    unittest.main()
