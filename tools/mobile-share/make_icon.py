#!/usr/bin/env python3
"""Draws the launcher icon. No dependencies, no font, no external assets."""
import math
import struct
import zlib
from pathlib import Path

SIZE = 192
BG = (23, 26, 33)
FG = (240, 246, 252)
ACCENT = (255, 122, 89)


def rounded(x, y, w, h, r):
    """True when (x, y) is inside a rounded rectangle anchored at 0,0."""
    if x < 0 or y < 0 or x >= w or y >= h:
        return False
    cx = r if x < r else (w - r - 1 if x >= w - r else x)
    cy = r if y < r else (h - r - 1 if y >= h - r else y)
    if cx == x or cy == y:
        return True
    return (x - cx) ** 2 + (y - cy) ** 2 <= r * r


def bookmark(x, y):
    """A bookmark ribbon: a rectangle with a V notch at the bottom."""
    left, right, top, bottom = 66, 126, 46, 150
    if not (left <= x <= right and top <= y <= bottom):
        return False
    notch = bottom - 26 + abs(x - (left + right) / 2) * 0.95
    return y <= notch or y <= bottom - 26


def blend(dst, src, alpha):
    return tuple(int(round(d + (s - d) * alpha)) for d, s in zip(dst, src))


def main():
    rows = []
    for y in range(SIZE):
        row = bytearray()
        for x in range(SIZE):
            if not rounded(x, y, SIZE, SIZE, 34):
                row += bytes((0, 0, 0, 0))
                continue
            px = BG
            # A soft accent circle behind the ribbon, clipped by the icon shape.
            if (x - 150) ** 2 + (y - 44) ** 2 < 52 ** 2:
                px = blend(px, ACCENT, 0.85)
            if bookmark(x, y):
                px = FG
            row += bytes((px[0], px[1], px[2], 255))
        rows.append(bytes(row))

    raw = b"".join(b"\x00" + r for r in rows)

    def chunk(tag, data):
        body = tag + data
        return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body) & 0xFFFFFFFF)

    png = (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", SIZE, SIZE, 8, 6, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(raw, 9))
        + chunk(b"IEND", b"")
    )

    out = Path(__file__).resolve().parent / "res" / "drawable" / "icon.png"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(png)
    print(f"{out} {len(png)} bytes")


if __name__ == "__main__":
    main()
