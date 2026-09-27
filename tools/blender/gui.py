"""GUI kit for OMMRPG, modelled in code and rendered with Blender.

A dark-fantasy kit in the spirit of classic MMO interfaces (carved iron
frames with bronze filigree corners, recessed stone panels, engraved metal
buttons, rarity-rimmed slots, glass "tube" bars): every part is a real 3D
model lit by the same rig, rendered at 2x and packed into one atlas with
its 9-slice margins (see pack.py), so the client can stretch it to any
size and it stays sharp on phones.

    python3 tools/blender/gui.py build/art
"""
import json
import math
import sys
from pathlib import Path

import bpy
from mathutils import Vector
from PIL import Image

sys.path.insert(0, str(Path(__file__).parent))
import common as C  # noqa: E402

PX = 128.0  # rendered pixels per Blender unit (2x the on-screen size)
SAMPLES = 40

# ---------------------------------------------------------------- scene

def scene(w_px, h_px):
    C.reset(int(w_px), int(h_px), samples=SAMPLES)
    sc = bpy.context.scene
    sc.view_settings.exposure = -0.35
    bpy.ops.object.camera_add(location=(0, -10, 0), rotation=(math.pi / 2, 0, 0))
    cam = bpy.context.object
    sc.camera = cam
    cam.data.type = 'ORTHO'
    cam.data.ortho_scale = max(w_px, h_px) / PX

    def area(loc, energy, color, size):
        bpy.ops.object.light_add(type='AREA', location=loc)
        light = bpy.context.object
        light.data.energy = energy
        light.data.color = color
        light.data.size = size
        light.rotation_euler = (-Vector(loc)).to_track_quat('-Z', 'Y').to_euler()
    area((-4, -6, 5), 2200, (1.0, 0.88, 0.72), 4)   # warm key, upper left
    area((6, -5, -2), 260, (0.5, 0.6, 0.8), 6)      # cool fill, lower right
    C.world(sky=(0.75, 0.62, 0.45), ground=(0.03, 0.025, 0.02), strength=0.35)


def mats():
    return {
        "iron": C.material("iron", (0.16, 0.15, 0.145), metal=1.0, rough=0.36, noise=0.3, noise_scale=30),
        "iron_dark": C.material("iron_dark", (0.06, 0.055, 0.052), metal=1.0, rough=0.45, noise=0.2, noise_scale=30),
        "bronze": C.material("bronze", (0.58, 0.36, 0.16), metal=1.0, rough=0.3, noise=0.2, noise_scale=40),
        "gold": C.material("gold", (0.95, 0.7, 0.32), metal=1.0, rough=0.26),
        "stone": C.material("stone", (0.014, 0.013, 0.012), rough=0.97, noise=0.4, noise_scale=3.5),
        "stone_dark": C.material("stone_dark", (0.007, 0.0065, 0.006), rough=0.98, noise=0.3, noise_scale=4),
        "ruby": C.material("ruby", (0.9, 0.06, 0.05), rough=0.08, emit=(0.8, 0.05, 0.03), strength=0.5, coat=1),
        "sapphire": C.material("sapph", (0.1, 0.35, 0.95), rough=0.08, emit=(0.1, 0.35, 0.9), strength=0.5, coat=1),
    }


def u(px):
    return px / PX


def box(x0, z0, x1, z1, depth, mat, y=0.0, bevel=0.0):
    """Box spanning [x0, x1] x [z0, z1] (px from the canvas centre), front
    face at y - depth / 2 (towards the camera)."""
    o = C.add("cube", size=1, location=(u(x0 + x1) / 2, y, u(z0 + z1) / 2))
    o.scale = (u(abs(x1 - x0)), depth, u(abs(z1 - z0)))
    return C.finish(o, mat, bevel=bevel, segments=3)


def frame(x0, z0, x1, z1, width, depth, mat, y=0.0, bevel=0.01):
    """Rectangular border of `width` px."""
    box(x0, z1 - width, x1, z1, depth, mat, y, bevel)
    box(x0, z0, x1, z0 + width, depth, mat, y, bevel)
    box(x0, z0 + width, x0 + width, z1 - width, depth, mat, y, bevel)
    box(x1 - width, z0 + width, x1, z1 - width, depth, mat, y, bevel)


def sphere(x, z, r, mat, y=-0.05, scale=(1, 0.6, 1)):
    o = C.add("uv_sphere", radius=u(r), location=(u(x), y, u(z)), segments=24, ring_count=12)
    o.scale = scale
    return C.finish(o, mat)


def gem(x, z, r, mat, y=-0.08):
    o = C.add("cone", vertices=8, radius1=u(r), radius2=u(r) * 0.55, depth=u(r) * 0.6, location=(u(x), y, u(z)))
    o.rotation_euler = (math.pi / 2, 0, 0)
    C.finish(o, mat, smooth=False)
    ring = C.add("torus", major_radius=u(r) * 1.05, minor_radius=u(r) * 0.22, location=(u(x), y + 0.02, u(z)))
    ring.rotation_euler = (math.pi / 2, 0, 0)
    C.finish(ring, bpy.data.materials["gold"])


def curl(cx, cz, r, start, turns, thick, mat, y=-0.04, flip=1):
    """A filigree scroll: a flat spiral tube."""
    cu = bpy.data.curves.new("curl", 'CURVE')
    cu.dimensions = '3D'
    cu.bevel_depth = u(thick)
    cu.bevel_resolution = 3
    sp = cu.splines.new('POLY')
    n = 48
    sp.points.add(n - 1)
    for i in range(n):
        t = i / (n - 1)
        a = start + flip * t * turns * math.tau
        rr = u(r) * (1 - 0.72 * t)
        sp.points[i].co = (u(cx) + math.cos(a) * rr, y, u(cz) + math.sin(a) * rr, 1)
    o = bpy.data.objects.new("curl", cu)
    bpy.context.collection.objects.link(o)
    o.data.materials.append(mat)
    return o


def diamond(x, z, r, depth, mat, y=-0.03):
    o = C.add("cube", size=1, location=(u(x), y, u(z)))
    o.scale = (u(r) * 1.41, depth, u(r) * 1.41)
    o.rotation_euler = (0, math.pi / 4, 0)
    return C.finish(o, mat, bevel=0.012, segments=3)


def corner_ornament(cx, cz, sx, sz, m, size=1.0):
    """Bronze bracket with filigree in one corner; (sx, sz) point towards
    the corner and (cx, cz) is the corner of the inner molding."""
    s = size
    # L bracket along both edges
    box(cx - sx * 62 * s, cz - sz * 5 * s, cx + sx * 8 * s, cz + sz * 9 * s, 0.12, m["bronze"], y=-0.04, bevel=0.012)
    box(cx - sx * 5 * s, cz - sz * 62 * s, cx + sx * 9 * s, cz + sz * 8 * s, 0.12, m["bronze"], y=-0.04, bevel=0.012)
    # scroll curls where the bracket ends
    curl(cx - sx * 66 * s, cz + sz * 2 * s, 9 * s, math.pi / 2 if sz > 0 else -math.pi / 2, 1.35, 2.8 * s, m["bronze"], y=-0.07, flip=-sx * sz)
    curl(cx + sx * 2 * s, cz - sz * 66 * s, 9 * s, 0 if sx > 0 else math.pi, 1.35, 2.8 * s, m["bronze"], y=-0.07, flip=sx * sz)
    # the boss: a diamond plate with a gem and an outward spike
    diamond(cx + sx * 2 * s, cz + sz * 2 * s, 24 * s, 0.14, m["bronze"], y=-0.07)
    diamond(cx + sx * 2 * s, cz + sz * 2 * s, 16 * s, 0.16, m["iron_dark"], y=-0.09)
    gem(cx + sx * 2 * s, cz + sz * 2 * s, 9 * s, m["ruby"], y=-0.16)
    for d in (1, 2, 3):
        sphere(cx - sx * (24 + d * 12) * s, cz + sz * 2 * s, 2.4 * s, m["gold"], y=-0.11)
        sphere(cx + sx * 2 * s, cz - sz * (24 + d * 12) * s, 2.4 * s, m["gold"], y=-0.11)


# ---------------------------------------------------------------- parts
# Each returns (width px, height px, 9-slice margins [l, t, r, b] in px).

def part_frame(m):
    W = H = 288
    hw, hh = W / 2, H / 2
    box(-hw + 8, -hh + 8, hw - 8, hh - 8, 0.04, m["stone"], y=0.08)
    frame(-hw + 6, -hh + 6, hw - 6, hh - 6, 22, 0.14, m["iron"], bevel=0.02)       # carved iron border
    frame(-hw + 10, -hh + 10, hw - 10, hh - 10, 3, 0.16, m["bronze"], y=-0.02, bevel=0.004)
    frame(-hw + 26, -hh + 26, hw - 26, hh - 26, 5, 0.12, m["bronze"], y=-0.01, bevel=0.006)  # inner molding
    frame(-hw + 31, -hh + 31, hw - 31, hh - 31, 3, 0.06, m["iron_dark"], y=0.02, bevel=0.002)
    for sx in (-1, 1):
        for sz in (-1, 1):
            corner_ornament(sx * (hw - 26), sz * (hh - 26), sx, sz, m)
    return W, H, [104, 104, 104, 104]


def part_inset(m):
    W = H = 96
    hw, hh = W / 2, H / 2
    box(-hw + 3, -hh + 3, hw - 3, hh - 3, 0.04, m["stone_dark"], y=0.06)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 5, 0.06, m["iron"], bevel=0.006)
    frame(-hw + 6, -hh + 6, hw - 6, hh - 6, 2, 0.04, m["bronze"], y=0.0, bevel=0.002)
    return W, H, [16, 16, 16, 16]


def part_plaque(m):
    W, H = 320, 72
    hw, hh = W / 2, H / 2
    box(-hw + 44, -hh + 10, hw - 44, hh - 10, 0.08, m["iron"], bevel=0.02)
    frame(-hw + 44, -hh + 10, hw - 44, hh - 10, 4, 0.1, m["bronze"], y=-0.01, bevel=0.004)
    for sx in (-1, 1):  # pointed end caps with a gem
        x = sx * (hw - 30)
        diamond(x, 0, 24, 0.1, m["bronze"])
        diamond(x, 0, 17, 0.12, m["iron"], y=-0.05)
        gem(x, 0, 8, m["ruby"])
        curl(x - sx * 8, 20, 9, math.pi / 2, 1.2, 2.2, m["bronze"], flip=-sx)
        curl(x - sx * 8, -20, 9, -math.pi / 2, 1.2, 2.2, m["bronze"], flip=sx)
    return W, H, [80, 24, 80, 24]


BUTTON_COLORS = {
    "gray": (0.2, 0.22, 0.26), "orange": (0.62, 0.38, 0.12), "red": (0.5, 0.08, 0.06),
    "green": (0.12, 0.35, 0.14), "blue": (0.1, 0.22, 0.5), "purple": (0.3, 0.12, 0.45),
}


def part_button(m, kind, state):
    W, H = 176, 64
    hw, hh = W / 2, H / 2
    base = BUTTON_COLORS[kind]
    metal = 0.85
    if state == "disabled":
        g = sum(base) / 3 * 0.45
        base = (g, g, g * 0.95)
        metal = 0.6
    face = C.material(f"face_{kind}_{state}", base, metal=metal, rough=0.38, noise=0.2, noise_scale=20)
    if state == "hover":
        b = face.node_tree.nodes["Principled BSDF"]
        b.inputs["Emission Color"].default_value = (*[min(1, c * 1.3) for c in base], 1)
        b.inputs["Emission Strength"].default_value = 0.12
    y = 0.04 if state == "pressed" else 0.0
    box(-hw + 4, -hh + 4, hw - 4, hh - 4, 0.12, face, y=y, bevel=0.045)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 4, 0.16, m["iron_dark"], y=0.02, bevel=0.01)
    frame(-hw + 9, -hh + 9, hw - 9, hh - 9, 2, 0.04, m["bronze"] if state != "disabled" else m["iron"], y=y - 0.06, bevel=0.002)
    for sx in (-1, 1):
        for sz in (-1, 1):
            sphere(sx * (hw - 9), sz * (hh - 9), 3.2, m["gold"] if state != "disabled" else m["iron"], y=-0.07 + y)
    return W, H, [24, 22, 24, 22]


RARITY = {
    "empty": (0.13, 0.12, 0.12), "common": (0.55, 0.55, 0.56), "uncommon": (0.2, 0.62, 0.28),
    "rare": (0.18, 0.4, 0.9), "epic": (0.55, 0.22, 0.85), "legendary": (0.95, 0.55, 0.12), "mythic": (0.9, 0.12, 0.12),
}


def part_slot(m, rarity):
    W = H = 128
    hw, hh = W / 2, H / 2
    rim = C.material(f"rim_{rarity}", RARITY[rarity], metal=1.0, rough=0.3, noise=0.15, noise_scale=30)
    box(-hw + 10, -hh + 10, hw - 10, hh - 10, 0.04, m["stone_dark"], y=0.1)
    frame(-hw + 2, -hh + 2, hw - 2, hh - 2, 11, 0.14, rim, bevel=0.014)
    frame(-hw + 13, -hh + 13, hw - 13, hh - 13, 3, 0.06, m["iron_dark"], y=0.04, bevel=0.002)
    frame(-hw + 2, -hh + 2, hw - 2, hh - 2, 2, 0.16, m["iron_dark"], y=-0.005, bevel=0.002)
    for sx in (-1, 1):
        for sz in (-1, 1):
            diamond(sx * (hw - 7), sz * (hh - 7), 6, 0.1, m["gold"], y=-0.06)
    return W, H, [24, 24, 24, 24]


def part_bar_frame(m):
    W, H = 256, 44
    hw, hh = W / 2, H / 2
    box(-hw + 6, -hh + 6, hw - 6, hh - 6, 0.04, m["stone_dark"], y=0.06)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 6, 0.1, m["iron"], bevel=0.01)
    frame(-hw + 7, -hh + 7, hw - 7, hh - 7, 1.5, 0.05, m["bronze"], y=0.0, bevel=0.0)
    for sx in (-1, 1):
        diamond(sx * (hw - 4), 0, 10, 0.12, m["bronze"], y=-0.05)
    return W, H, [20, 14, 20, 14]


BAR_COLORS = {"red": (0.85, 0.1, 0.08), "blue": (0.12, 0.4, 0.95), "green": (0.15, 0.75, 0.25),
              "gold": (0.95, 0.65, 0.12), "purple": (0.6, 0.2, 0.9)}


def part_bar_fill(m, color):
    W, H = 96, 32
    c = BAR_COLORS[color]
    glass = C.material(f"tube_{color}", c, rough=0.15, emit=c, strength=0.18, coat=1.0)
    o = C.add("cylinder", vertices=32, radius=u(H / 2 - 1), depth=u(W + 40), location=(0, 0, 0))
    o.rotation_euler = (0, math.pi / 2, 0)
    o.scale = (1, 0.6, 1)
    C.finish(o, glass)
    return W, H, [8, 0, 8, 0]


def part_tab(m, active):
    W, H = 176, 60
    hw, hh = W / 2, H / 2
    face = m["bronze"] if active else m["iron"]
    box(-hw + 4, -hh, hw - 4, hh - 4, 0.12, face, bevel=0.04)
    frame(-hw + 1, -hh - 10, hw - 1, hh - 1, 4, 0.16, m["iron_dark"], y=0.02, bevel=0.01)
    if active:  # bronze lip along the top edge marks the open tab
        box(-hw + 16, hh - 9, hw - 16, hh - 5, 0.14, m["gold"], y=-0.04, bevel=0.004)
    return W, H, [26, 20, 26, 8]


def part_pill(m):
    W, H = 192, 52
    hw, hh = W / 2, H / 2
    body = C.add("cylinder", vertices=48, radius=u(hh - 2), depth=0.1, location=(0, 0.05, 0))
    body.rotation_euler = (math.pi / 2, 0, 0)
    body.scale = (u(W - 4) / (2 * u(hh - 2)), 1, 1)
    C.finish(body, m["stone_dark"])
    rim = C.add("torus", major_radius=u(hh - 4), minor_radius=u(3), location=(0, 0, 0))
    rim.rotation_euler = (math.pi / 2, 0, 0)
    rim.scale = (u(W - 8) / (2 * u(hh - 4)), 1, 1)
    C.finish(rim, m["iron"])
    return W, H, [28, 20, 28, 20]


def part_ring(m, filled):
    W = H = 256
    ring = C.add("torus", major_radius=u(112), minor_radius=u(12), location=(0, 0, 0), major_segments=96, minor_segments=24)
    ring.rotation_euler = (math.pi / 2, 0, 0)
    C.finish(ring, m["bronze"])
    inner = C.add("torus", major_radius=u(99), minor_radius=u(4), location=(0, -0.02, 0), major_segments=96)
    inner.rotation_euler = (math.pi / 2, 0, 0)
    C.finish(inner, m["iron"])
    if filled:
        disc = C.add("cylinder", vertices=96, radius=u(100), depth=0.05, location=(0, 0.08, 0))
        disc.rotation_euler = (math.pi / 2, 0, 0)
        C.finish(disc, m["stone_dark"])
    for i in range(4):
        a = i * math.pi / 2 + math.pi / 4
        x, z = math.cos(a) * 112, math.sin(a) * 112
        diamond(x, z, 12, 0.1, m["gold"], y=-0.07)
    for i in range(4):
        a = i * math.pi / 2
        x, z = math.cos(a) * 112, math.sin(a) * 112
        gem(x, z, 8, m["ruby"] if i % 2 else m["sapphire"], y=-0.1)
    return W, H, None


def part_close(m):
    W = H = 96
    disc = C.add("cylinder", vertices=48, radius=u(38), depth=0.1, location=(0, 0, 0))
    disc.rotation_euler = (math.pi / 2, 0, 0)
    C.finish(disc, C.material("closeface", (0.5, 0.06, 0.05), metal=0.8, rough=0.35), bevel=0.03)
    ring = C.add("torus", major_radius=u(40), minor_radius=u(6), location=(0, -0.03, 0), major_segments=48)
    ring.rotation_euler = (math.pi / 2, 0, 0)
    C.finish(ring, m["bronze"])
    for a in (math.pi / 4, -math.pi / 4):
        bar = C.add("cube", size=1, location=(0, -0.08, 0))
        bar.scale = (u(44), 0.05, u(9))
        bar.rotation_euler = (0, a, 0)
        C.finish(bar, m["gold"], bevel=0.01)
    return W, H, None


def part_separator(m):
    W, H = 512, 28
    box(-236, -3, 236, 3, 0.05, m["bronze"], bevel=0.006)
    box(-236, -1, 236, 1, 0.06, m["gold"], y=-0.01)
    diamond(0, 0, 11, 0.1, m["bronze"], y=-0.03)
    gem(0, 0, 6, m["ruby"])
    for sx in (-1, 1):
        diamond(sx * 242, 0, 8, 0.08, m["bronze"])
        curl(sx * 30, 0, 8, math.pi if sx > 0 else 0, 1.1, 2, m["bronze"], flip=sx)
    return W, H, [48, 0, 48, 0]


PARTS = {"frame": part_frame, "inset": part_inset, "plaque": part_plaque, "bar_frame": part_bar_frame,
         "pill": part_pill, "separator": part_separator, "close": part_close,
         "ring": lambda m: part_ring(m, False), "ring_filled": lambda m: part_ring(m, True),
         "tab": lambda m: part_tab(m, False), "tab_on": lambda m: part_tab(m, True)}
for _k in BUTTON_COLORS:
    for _s in ("normal", "hover", "pressed", "disabled"):
        PARTS[f"button_{_k}_{_s}"] = (lambda k, s: (lambda m: part_button(m, k, s)))(_k, _s)
for _r in RARITY:
    PARTS[f"slot_{_r}"] = (lambda r: (lambda m: part_slot(m, r)))(_r)
for _c in BAR_COLORS:
    PARTS[f"bar_{_c}"] = (lambda c: (lambda m: part_bar_fill(m, c)))(_c)


def render_part(name, raw):
    # the size is decided by the part itself: build once to learn it
    C.reset(8, 8)
    w, h, margins = PARTS[name](mats())
    scene(w, h)
    PARTS[name](mats())
    path = raw / f"{name}.png"
    C.render(path)
    return {"size": [w, h], "margin": margins}


def pack_atlas(raw, meta, out):
    """Shelf-pack the parts into one atlas (2 px gutter)."""
    items = sorted(meta.items(), key=lambda kv: -kv[1]["size"][1])
    width = 1024
    x = y = shelf = 0
    rects = {}
    for name, m in items:
        w, h = m["size"]
        if x + w > width:
            x, y, shelf = 0, y + shelf + 2, 0
        rects[name] = [x, y, w, h]
        x += w + 2
        shelf = max(shelf, h)
    atlas = Image.new("RGBA", (width, y + shelf), (0, 0, 0, 0))
    for name, (x, y, w, h) in rects.items():
        atlas.alpha_composite(Image.open(raw / f"{name}.png").convert("RGBA"), (x, y))
    out.mkdir(parents=True, exist_ok=True)
    atlas.save(out / "gui.png", optimize=True)
    parts = {n: {"rect": rects[n], "margin": meta[n]["margin"]} for n in meta}
    (out / "gui.json").write_text(json.dumps({"scale": 0.5, "parts": parts}, indent=1))
    print("gui atlas", atlas.size, len(parts), "parts")


def main():
    root = Path(sys.argv[1] if len(sys.argv) > 1 else "build/art")
    only = sys.argv[2:]
    raw = root / "raw_gui"
    raw.mkdir(parents=True, exist_ok=True)
    meta_path = raw / "gui.json"
    meta = json.loads(meta_path.read_text()) if meta_path.exists() else {}
    for name in PARTS:
        if only and name not in only:
            continue
        print("gui", name, flush=True)
        meta[name] = render_part(name, raw)
    meta = {k: v for k, v in meta.items() if k in PARTS}
    meta_path.write_text(json.dumps(meta, indent=1))
    pack_atlas(raw, meta, root / "pack")


if __name__ == "__main__":
    main()
