#!/usr/bin/env python3

from __future__ import annotations

import argparse
import shutil
from collections import deque
from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
ASSETS_DIR = ROOT / "assets" / "icons"
WEB_ASSETS_DIR = ROOT / "web" / "static" / "assets"
ICONSET_DIR = ASSETS_DIR / "agent-team-monitor.iconset"

SOURCE_FILES = {
    "icon_16x16.png": "agent-team-monitor-16.png",
    "icon_16x16@2x.png": "agent-team-monitor-32.png",
    "icon_32x32@2x.png": "agent-team-monitor-64.png",
    "icon_128x128.png": "agent-team-monitor-128.png",
    "icon_256x256.png": "agent-team-monitor-256.png",
    "icon_512x512.png": "agent-team-monitor.png",
    "icon_512x512@2x.png": "agent-team-monitor-1024.png",
}

ICO_SIZES = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
BACKGROUND_TOLERANCE = 40


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate repo icon assets from a macOS AppIcon.iconset directory."
    )
    parser.add_argument("source", type=Path, help="Path to AppIcon.iconset")
    return parser.parse_args()


def ensure_sources(source_dir: Path) -> None:
    missing = sorted(name for name in SOURCE_FILES if not (source_dir / name).is_file())
    if missing:
        raise SystemExit(
            "missing iconset files:\n" + "\n".join(f"  - {source_dir / name}" for name in missing)
        )


def is_edge_background(pixel: tuple[int, int, int, int], tolerance: int = BACKGROUND_TOLERANCE) -> bool:
    r, g, b, a = pixel
    if a == 0:
        return True
    return (
        abs(r - 255) <= tolerance
        and abs(g - 255) <= tolerance
        and abs(b - 255) <= tolerance
    )


def strip_edge_background(image: Image.Image) -> Image.Image:
    rgba = image.convert("RGBA")
    width, height = rgba.size
    pixels = rgba.load()
    background = bytearray(width * height)
    queue: deque[tuple[int, int]] = deque()

    def enqueue_if_background(x: int, y: int) -> None:
        index = y * width + x
        if background[index]:
            return
        if not is_edge_background(pixels[x, y]):
            return
        background[index] = 1
        queue.append((x, y))

    for x in range(width):
        enqueue_if_background(x, 0)
        enqueue_if_background(x, height - 1)
    for y in range(height):
        enqueue_if_background(0, y)
        enqueue_if_background(width - 1, y)

    while queue:
        x, y = queue.popleft()
        for next_x, next_y in ((x - 1, y), (x + 1, y), (x, y - 1), (x, y + 1)):
            if 0 <= next_x < width and 0 <= next_y < height:
                enqueue_if_background(next_x, next_y)

    output = rgba.copy()
    output_pixels = output.load()
    for y in range(height):
        row_offset = y * width
        for x in range(width):
            if background[row_offset + x]:
                output_pixels[x, y] = (0, 0, 0, 0)

    return output


def processed_icon(source_path: Path) -> Image.Image:
    return strip_edge_background(Image.open(source_path))


def copy_png_assets(source_dir: Path) -> None:
    ASSETS_DIR.mkdir(parents=True, exist_ok=True)
    WEB_ASSETS_DIR.mkdir(parents=True, exist_ok=True)

    for source_name, dest_name in SOURCE_FILES.items():
        processed_icon(source_dir / source_name).save(ASSETS_DIR / dest_name)

    processed_icon(source_dir / "icon_16x16@2x.png").save(WEB_ASSETS_DIR / "favicon.png")
    processed_icon(source_dir / "icon_512x512.png").save(WEB_ASSETS_DIR / "agent-team-monitor.png")


def copy_iconset(source_dir: Path) -> None:
    if ICONSET_DIR.exists():
        shutil.rmtree(ICONSET_DIR)
    ICONSET_DIR.mkdir(parents=True, exist_ok=True)

    for entry in source_dir.iterdir():
        destination = ICONSET_DIR / entry.name
        if entry.is_file() and entry.suffix.lower() == ".png":
            processed_icon(entry).save(destination)
            continue
        if entry.is_file():
            shutil.copy2(entry, destination)


def save_platform_icons(source_dir: Path) -> None:
    master = processed_icon(source_dir / "icon_512x512@2x.png")
    master.save(ASSETS_DIR / "agent-team-monitor.ico", format="ICO", sizes=ICO_SIZES)
    master.save(ASSETS_DIR / "agent-team-monitor.icns", format="ICNS")


def main() -> None:
    args = parse_args()
    source_dir = args.source.expanduser().resolve()

    if not source_dir.is_dir():
        raise SystemExit(f"iconset directory not found: {source_dir}")

    ensure_sources(source_dir)
    copy_png_assets(source_dir)
    copy_iconset(source_dir)
    save_platform_icons(source_dir)

    print(f"updated icons from {source_dir}")
    print(f"  png assets: {ASSETS_DIR}")
    print(f"  web assets: {WEB_ASSETS_DIR}")
    print(f"  iconset:    {ICONSET_DIR}")
    print(f"  ico/icns:   {ASSETS_DIR / 'agent-team-monitor.ico'}")


if __name__ == "__main__":
    main()
