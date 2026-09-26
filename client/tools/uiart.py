#!/usr/bin/env python3
"""Generates the bitmap parts of the UI kit (requires Pillow).

Most of the UI is drawn in code (scenes/ui/fancy_box.gd) so it stays sharp
at any resolution; only textures that need noise live here:

  assets/ui/stone.png   tileable dungeon stone wall (screen backgrounds)
  assets/ui/floor.png   tileable flagstone floor (hero screen pedestal)
  assets/ui/switch_on.png, switch_off.png   toggle switches (CheckButton)

    python3 client/tools/uiart.py
"""
import random
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

OUT = Path(__file__).resolve().parent.parent / "assets" / "ui"
SIZE = 256


def noise_layer(rng, size, lo, hi, blur):
    img = Image.new("L", (size, size))
    img.putdata([rng.randint(lo, hi) for _ in range(size * size)])
    # blur with wrap-around so the texture stays tileable
    big = Image.new("L", (size * 3, size * 3))
    for x in range(3):
        for y in range(3):
            big.paste(img, (x * size, y * size))
    big = big.filter(ImageFilter.GaussianBlur(blur))
    return big.crop((size, size, size * 2, size * 2))


def bricks(seed, row_h, min_w, max_w, base, mortar, jitter):
    rng = random.Random(seed)
    img = Image.new("RGB", (SIZE, SIZE), mortar)
    fine = noise_layer(rng, SIZE, 0, 255, 0.6)
    coarse = noise_layer(rng, SIZE, 0, 255, 6)
    for row in range(SIZE // row_h):
        y0 = row * row_h
        # split the row into bricks whose widths sum to SIZE (tileable)
        widths, left = [], SIZE
        while left > 0:
            w = min(left, rng.randint(min_w, max_w))
            if 0 < left - w < min_w:
                w = left
            widths.append(w)
            left -= w
        x = rng.randint(0, SIZE - 1)
        for w in widths:
            k = rng.uniform(-jitter, jitter)
            col = tuple(max(0, min(255, int(c * (1 + k)))) for c in base)
            for dx in range(w):
                px = (x + dx) % SIZE
                for dy in range(2, row_h - 1):
                    py = y0 + dy
                    edge = dx < 2 or dx > w - 3
                    n = (fine.getpixel((px, py)) - 128) / 128 * 0.12 + (coarse.getpixel((px, py)) - 128) / 128 * 0.22
                    shade = 1 + n
                    if dy < 4:
                        shade += 0.18  # lit top edge
                    elif dy > row_h - 4:
                        shade -= 0.25  # shadowed bottom edge
                    if edge:
                        shade -= 0.2
                    img.putpixel((px, py), tuple(max(0, min(255, int(c * shade))) for c in col))
            x += w
    # stains and chips
    stains = noise_layer(rng, SIZE, 0, 255, 10)
    for px in range(SIZE):
        for py in range(SIZE):
            v = stains.getpixel((px, py))
            if v < 118:
                c = img.getpixel((px, py))
                k = 0.72 + (v - 100) / 18 * 0.28
                img.putpixel((px, py), tuple(int(ch * max(0.72, min(1, k))) for ch in c))
    return img


def switch(on, path):
    k = 4  # supersample
    w, h = 64 * k, 36 * k
    img = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.rounded_rectangle((0, 0, w - 1, h - 1), radius=h // 2, fill=(13, 9, 8, 255))
    track = (46, 160, 74, 255) if on else (58, 50, 46, 255)
    d.rounded_rectangle((3 * k, 3 * k, w - 3 * k, h - 3 * k), radius=h // 2 - 3 * k, fill=track)
    d.rounded_rectangle((3 * k, 3 * k, w - 3 * k, h // 2), radius=h // 2 - 3 * k, fill=tuple(min(255, c + 30) for c in track[:3]) + (255,))
    cx = w - h // 2 if on else h // 2
    r = h // 2 - 5 * k
    d.ellipse((cx - r, h // 2 - r + k, cx + r, h // 2 + r + k), fill=(0, 0, 0, 110))
    d.ellipse((cx - r, h // 2 - r, cx + r, h // 2 + r), fill=(13, 9, 8, 255))
    r2 = r - 2 * k
    knob = (255, 232, 170, 255) if on else (190, 176, 160, 255)
    d.ellipse((cx - r2, h // 2 - r2, cx + r2, h // 2 + r2), fill=knob)
    img.resize((64, 36), Image.LANCZOS).save(path)


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    bricks(7, 32, 44, 92, (60, 52, 47), (16, 12, 11), 0.2).save(OUT / "stone.png")
    bricks(11, 64, 60, 128, (72, 62, 54), (24, 19, 17), 0.08).save(OUT / "floor.png")
    switch(True, OUT / "switch_on.png")
    switch(False, OUT / "switch_off.png")
    print("wrote", OUT)


if __name__ == "__main__":
    main()
