"""Turns the 4x Blender renders into pixel art and packs the atlas.

For each prop: box-downscale to 1x (premultiplied, so edges don't go
dark), hard alpha, a limited palette, and a 1 px outline in a darker shade
of the neighbouring colour, like hand-made LPC art. The props are then
packed into one atlas on a 32 px grid; `props.json` tells the client each
variant's cell rectangle (in tiles).

    python3 tools/blender/pixelate.py build/art
"""
import json
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageEnhance

SUPERSAMPLE = 4
TILE = 32
ATLAS_COLS = 24  # tiles per atlas row
COLORS = 64


def downscale(img):
    w, h = img.size[0] // SUPERSAMPLE, img.size[1] // SUPERSAMPLE
    # premultiply so transparent black does not bleed into the edge colours
    r, g, b, a = img.split()
    pre = Image.merge("RGB", [Image.composite(c, Image.new("L", img.size, 0), a) for c in (r, g, b)])
    small_rgb = pre.resize((w, h), Image.BOX)
    small_a = a.resize((w, h), Image.BOX)
    px_rgb, px_a = small_rgb.load(), small_a.load()
    out = Image.new("RGBA", (w, h))
    po = out.load()
    for y in range(h):
        for x in range(w):
            al = px_a[x, y]
            if al < 110:
                continue
            k = 255.0 / max(al, 1)
            c = px_rgb[x, y]
            po[x, y] = (min(255, int(c[0] * k)), min(255, int(c[1] * k)), min(255, int(c[2] * k)), 255)
    return out


def posterize(img):
    rgb = ImageEnhance.Color(img.convert("RGB")).enhance(1.25)
    rgb = ImageEnhance.Contrast(rgb).enhance(1.08)
    q = rgb.quantize(COLORS, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.NONE).convert("RGB")
    q.putalpha(img.split()[3])
    return q


def outline(img):
    w, h = img.size
    src = img.load()
    out = img.copy()
    po = out.load()
    for y in range(h):
        for x in range(w):
            if src[x, y][3]:
                continue
            best = None
            for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                nx, ny = x + dx, y + dy
                if 0 <= nx < w and 0 <= ny < h and src[nx, ny][3]:
                    best = src[nx, ny]
                    break
            if best:
                po[x, y] = (best[0] // 4 + 8, best[1] // 4 + 6, best[2] // 4 + 8, 255)
    return out


def with_shadow(img, base):
    """Soft oval shadow under the prop, centred on its own cell."""
    w, h = img.size
    cx = base[0] * TILE + TILE / 2
    cy = base[1] * TILE + TILE / 2
    a = img.split()[3]
    # width of the opaque part just around the base
    band = range(max(0, int(cy) - 8), min(h, int(cy) + 12))
    cols = [x for x in range(w) if any(a.getpixel((x, y)) for y in band)]
    span = (max(cols) - min(cols) + 1) if cols else TILE * 0.6
    sw = max(14.0, min(w - 2.0, span * 1.1 + 4))
    shadow = Image.new("RGBA", img.size, (0, 0, 0, 0))
    ImageDraw.Draw(shadow).ellipse((cx - sw / 2, cy - 2, cx + sw / 2, cy + 9), fill=(10, 18, 10, 72))
    shadow.alpha_composite(img)
    return shadow


def pixelate(path, base, flat=False):
    img = outline(posterize(downscale(Image.open(path).convert("RGBA"))))
    return img if flat else with_shadow(img, base)


def main():
    root = Path(sys.argv[-1] if len(sys.argv) > 1 else "build/art")
    raw = root / "raw_props"
    meta = json.loads((raw / "props.json").read_text())
    sprites = []
    for obj_id, m in sorted(meta.items(), key=lambda kv: int(kv[0])):
        for v, f in enumerate(m["variants"]):
            flat = m["name"] in ("stairs_down", "flowers", "bones")
            img = pixelate(raw / f"{m['name']}_{v}.png", f["base"], flat)
            sprites.append((int(obj_id), v, f["size"][0], f["size"][1], f["base"], img))
    # shelf packing on the tile grid
    x = y = shelf = 0
    layout = {}
    placed = []
    for obj_id, v, w, h, base, img in sorted(sprites, key=lambda s: -s[3]):
        if x + w > ATLAS_COLS:
            x, y, shelf = 0, y + shelf, 0
        placed.append((x, y, img))
        layout.setdefault(str(obj_id), []).append({"at": [x, y], "size": [w, h], "base": base, "v": v})
        x += w
        shelf = max(shelf, h)
    rows = y + shelf
    atlas = Image.new("RGBA", (ATLAS_COLS * TILE, rows * TILE), (0, 0, 0, 0))
    for px, py, img in placed:
        atlas.alpha_composite(img, (px * TILE, py * TILE))
    out = root / "pack"
    out.mkdir(parents=True, exist_ok=True)
    atlas.save(out / "props.png", optimize=True)
    for k in layout:
        layout[k].sort(key=lambda e: e.pop("v"))
    (out / "props.json").write_text(json.dumps({"tile": TILE, "objects": layout}, indent=1))
    print("atlas", atlas.size, "sprites", len(sprites))


if __name__ == "__main__":
    main()
