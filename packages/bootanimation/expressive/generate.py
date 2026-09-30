#!/usr/bin/env python3
# SPDX-FileCopyrightText: The uwuAOSP Project
# SPDX-License-Identifier: Apache-2.0
# Spring/rotation parameters adapted from Material Components LoadingIndicatorAnimatorDelegate.
# Copyright 2024 The Android Open Source Project. Licensed under Apache-2.0.

"""Render the approved six-shape animation as AOSP dynamic-color channel masks.

Requires Pillow. Android consumes the prebuilt ZIPs; no Python runs during boot/build.
The geometry is traced from the user's references and saved in shapes.json.
"""

import argparse
import io
import json
import math
import os
from pathlib import Path
import tempfile
import zipfile

from PIL import Image, ImageDraw

FPS = 60
SEGMENT_SECONDS = 0.650
FRAMES_PER_SEGMENT = 39
INTRO_FRAMES = 4 * FRAMES_PER_SEGMENT
# 6 shapes * 140 degrees = 840 degrees. Three shape cycles close rotation at 2520 degrees.
LOOP_FRAMES = 3 * 6 * FRAMES_PER_SEGMENT
CHANNELS = (0, 1, 2, 3, 1, 2)
FALLBACK_COLORS = ("ADC6FF", "D0BCFF", "80D5E8", "839DFF")
ROOT = Path(__file__).resolve().parent


def solve_spring(x, velocity, seconds):
    omega = math.sqrt(200)
    decay = 0.6 * omega
    frequency = omega * math.sqrt(1 - 0.6 ** 2)
    offset = x - 1
    b = (velocity + decay * offset) / frequency
    envelope = math.exp(-decay * seconds)
    cosine, sine = math.cos(frequency * seconds), math.sin(frequency * seconds)
    return (1 + envelope * (offset * cosine + b * sine),
            envelope * ((b * frequency - decay * offset) * cosine
                        + (-offset * frequency - decay * b) * sine))


def spring_states():
    startup = [(0.0, 0.0)]
    current = startup[0]
    for i in range(40):
        x, velocity = solve_spring(*current, SEGMENT_SECONDS)
        current = (x - 1, velocity)
        if i < 3:
            startup.append(current)
    return startup, current


STARTUP, PERIODIC = spring_states()


def load_shapes():
    shapes = json.loads((ROOT / "shapes.json").read_text())
    if len(shapes) != 6 or any(len(s) != 240 for s in shapes):
        raise ValueError("Expected six approved contours with 240 radial samples each")
    if any(not math.isfinite(r) or not 0 < r <= 76.001 for s in shapes for r in s):
        raise ValueError("Invalid radial contour")
    return shapes


def motion_at(seconds, shapes, periodic=False):
    phase = seconds / SEGMENT_SECONDS
    # Frame timestamps may divide to 21.999999999999996 at a target boundary.
    if abs(phase - round(phase)) < 1e-10:
        phase = float(round(phase))
    step = math.floor(phase)
    initial = PERIODIC if periodic or step >= len(STARTUP) else STARTUP[step]
    progress, _ = solve_spring(*initial, (phase - step) * SEGMENT_SECONDS)
    position = step + progress
    shape_step = math.floor(position)
    index = shape_step % len(shapes)
    fraction = position - shape_step
    radii = [a + (b - a) * fraction
             for a, b in zip(shapes[index], shapes[(index + 1) % len(shapes)])]
    # Contour overshoot and color transition are intentionally independent, as in Material.
    color_fraction = min(1, max(0, progress))
    weights = [0.0] * 4
    weights[CHANNELS[step % 6]] += 1 - color_fraction
    weights[CHANNELS[(step + 1) % 6]] += color_fraction
    return radii, math.radians(50 * phase + 90 * position), weights


def render_mask(seconds, shapes, size, periodic=False):
    radii, rotation, weights = motion_at(seconds, shapes, periodic)
    supersampling = 3
    large_size = size * supersampling
    scale = large_size / 200
    points = [(large_size / 2 + math.cos(i * math.tau / 240 - math.pi / 2 + rotation)
               * r * scale,
               large_size / 2 + math.sin(i * math.tau / 240 - math.pi / 2 + rotation)
               * r * scale) for i, r in enumerate(radii)]
    coverage = Image.new("L", (large_size, large_size))
    ImageDraw.Draw(coverage).polygon(points, fill=255)
    coverage = coverage.resize((size, size), Image.Resampling.LANCZOS)
    # Alpha is color4, NOT transparency. Preserve RGB even when alpha is zero.
    # No pixel uses all four channels: that would invoke AOSP's special white-mask branch.
    return Image.merge("RGBA", [coverage.point([round(v * w) for v in range(256)])
                                 for w in weights])


def write_entry(archive, name, data):
    entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
    entry.compress_type = zipfile.ZIP_STORED
    entry.external_attr = 0o100644 << 16
    archive.writestr(entry, data)


def generate(resolution, output_dir, shapes):
    size = {720: 128, 1080: 192, 1440: 256}[resolution]
    offset = (resolution - size) // 2
    desc = (f"{resolution} {resolution} {FPS}\n"
            + "dynamic_colors intro " + " ".join("#" + c for c in FALLBACK_COLORS)
            + " 0 30\n"
            + "p 1 0 intro #000000\n"
            + "f 0 0 loop 12 #000000\n")
    target = output_dir / f"bootanimation_{resolution}.zip"
    fd, temporary = tempfile.mkstemp(prefix=target.name + ".", dir=output_dir)
    os.close(fd)
    try:
        with zipfile.ZipFile(temporary, "w") as archive:
            write_entry(archive, "desc.txt", desc.encode())
            for part, count in (("intro", INTRO_FRAMES), ("loop", LOOP_FRAMES)):
                for frame in range(count):
                    t = frame / FPS + (INTRO_FRAMES / FPS if part == "loop" else 0)
                    image = render_mask(t, shapes, size, periodic=part == "loop")
                    png = io.BytesIO()
                    image.save(png, format="PNG", compress_level=9)
                    write_entry(archive, f"{part}/{frame:05d}.png", png.getvalue())
                trim = f"{size}x{size}+{offset}+{offset}\n" * count
                write_entry(archive, f"{part}/trim.txt", trim.encode())
        os.replace(temporary, target)
        target.chmod(0o644)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    print(f"{target}: {target.stat().st_size:,} bytes, {INTRO_FRAMES + LOOP_FRAMES} frames",
          flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, default=ROOT)
    parser.add_argument("--resolutions", nargs="+", type=int, choices=(720, 1080, 1440),
                        default=[720, 1080, 1440])
    args = parser.parse_args()
    args.output_dir.mkdir(parents=True, exist_ok=True)
    shapes = load_shapes()
    for resolution in args.resolutions:
        generate(resolution, args.output_dir, shapes)


if __name__ == "__main__":
    main()
