"""Scale check: the packed props next to a real LPC hero on grass.

    python3 tools/blender/preview.py build/art http://localhost:8080 out.png
"""
import base64
import io
import json
import sys
import urllib.request
from pathlib import Path

from PIL import Image

root, api, out = Path(sys.argv[1]), sys.argv[2], sys.argv[3]
recipe = json.load(urllib.request.urlopen(f"{api}/api/v1/sprites/random?seed=7&body=male"))["recipe"]
enc = base64.urlsafe_b64encode(json.dumps(recipe).encode()).decode().rstrip("=")
sheet = Image.open(io.BytesIO(urllib.request.urlopen(f"{api}/api/v1/sprites/character.png?r={enc}").read())).convert("RGBA")
hero = sheet.crop((0, 10 * 64, 64, 11 * 64))  # walk, facing down, standing frame
atlas = Image.open(root / "pack" / "props.png")
layout = json.loads((root / "pack" / "props.json").read_text())
T = layout["tile"]
W, H = 30, 11
canvas = Image.new("RGBA", (W * T, H * T), (86, 150, 70, 255))
for x in range(W):  # checker so tiles are visible
    for y in range(H):
        if (x + y) % 2:
            canvas.paste((80, 142, 64, 255), (x * T, y * T, x * T + T, y * T + T))
entries = [v[0] for _, v in sorted(layout["objects"].items(), key=lambda kv: int(kv[0]))]
rows = [4, 9]  # base rows of the two lines
cx, line = 1, 0
first = True
for e in entries:
    ax, ay = e["at"]
    w, h = e["size"]
    bx, by = e["base"]
    if cx + w > W:
        cx, line = 1, line + 1
        if line >= len(rows):
            break
    sprite = atlas.crop((ax * T, ay * T, (ax + w) * T, (ay + h) * T))
    canvas.alpha_composite(sprite, (cx * T, (rows[line] - by) * T))
    cx += w
    if first:
        canvas.alpha_composite(hero, (cx * T - 16, rows[line] * T - 32 - 12))
        cx += 1
        first = False
canvas = canvas.resize((canvas.width * 2, canvas.height * 2), Image.NEAREST)
canvas.save(out)
