"""World props for OMMRPG, modelled in code and rendered with Blender.

Like Dead Cells, the art is 3D and turned into pixel art afterwards
(pixelate.py): every prop is rendered at 4x with the same sun and camera,
then downscaled, outlined and packed into one atlas.

Sizes follow the character: a tile is 32 px and 1 Blender unit; an LPC
hero is about 1.5 tiles tall, so an oak is 3x4 tiles and a pine 2x4.

    python3 tools/blender/props.py build/art   # needs `pip install bpy`
"""
import json
import math
import random
import sys
from pathlib import Path

import bpy  # noqa: F401  (loads mathutils)
from mathutils import Matrix, Vector

sys.path.insert(0, str(Path(__file__).parent))
import common as C  # noqa: E402

SUPERSAMPLE = 4
TILE_PX = 32
ELEVATION = 45
# models are built at "real" size (a hero is ~1.7 units tall); the camera
# tilt shortens heights, so everything is scaled up by the same factor
WORLD_SCALE = 1.3

# object id (world.Object) -> (name, width tiles, height tiles, variants)
PROPS = {
    1: ("tree", 3, 4, 3), 2: ("pine", 2, 4, 3), 3: ("rock", 1, 1, 3), 4: ("bush", 1, 1, 3),
    5: ("flowers", 1, 1, 3), 6: ("cactus", 1, 2, 2), 7: ("dead_tree", 2, 3, 2), 8: ("boulder", 2, 2, 2),
    9: ("reeds", 1, 2, 2), 10: ("dungeon_entrance", 3, 3, 1), 11: ("stairs_down", 1, 1, 1),
    12: ("dungeon_exit", 2, 3, 1), 13: ("chest", 1, 1, 1), 14: ("torch", 1, 2, 1), 15: ("anvil", 1, 1, 1),
    16: ("shrine", 2, 3, 1), 17: ("bones", 1, 1, 2),
}

# ---------------------------------------------------------------- materials

def mats(r):
    hue = r.uniform(-0.05, 0.05)
    return {
        "bark": C.material("bark", (0.32 + hue, 0.2, 0.11), rough=0.9, noise=0.35, noise_scale=12),
        "leaf": C.material("leaf", (0.2 + hue, 0.5 + hue * 2, 0.16), rough=0.7, noise=0.45, noise_scale=5),
        "leaf_dark": C.material("leaf_dark", (0.16, 0.5 + hue, 0.26), rough=0.85, noise=0.45, noise_scale=5),
        "stone": C.material("stone", (0.48, 0.46, 0.44), rough=0.85, noise=0.3, noise_scale=8),
        "stone_dark": C.material("stone_dark", (0.3, 0.29, 0.3), rough=0.9, noise=0.3, noise_scale=8),
        "moss": C.material("moss", (0.25, 0.45, 0.15), rough=0.9, noise=0.4, noise_scale=10),
        "wood": C.material("wood", (0.5, 0.26, 0.1), rough=0.6, noise=0.25, noise_scale=14),
        "iron": C.material("iron", (0.3, 0.3, 0.32), metal=1.0, rough=0.4),
        "gold": C.material("gold", (1.0, 0.7, 0.25), metal=1.0, rough=0.25),
        "bone": C.material("bone", (0.9, 0.86, 0.74), rough=0.6),
        "dark": C.material("dark", (0.02, 0.015, 0.02), rough=1.0),
        "fire": C.material("fire", (1.0, 0.45, 0.08), emit=(1.0, 0.42, 0.06), strength=3),
        "magic": C.material("magic", (0.15, 0.45, 0.95), emit=(0.15, 0.5, 1.0), strength=1.3, rough=0.2),
        "pine": C.material("pine", (0.07, 0.3 + hue, 0.13), rough=0.9, noise=0.5, noise_scale=7),
        "cactus": C.material("cactus", (0.24, 0.5, 0.24), rough=0.6, noise=0.2, noise_scale=20),
        "reed": C.material("reed", (0.45, 0.55, 0.22), rough=0.7, noise=0.3),
    }

# ---------------------------------------------------------------- models

def tree(r, m):
    trunk = C.add("cone", vertices=10, radius1=0.2, radius2=0.12, depth=1.5, location=(0, 0, 0.75))
    C.finish(C.displace(trunk, 0.04, 0.2, r.random() * 99), m["bark"])
    for a in range(3):  # roots
        ang = a * 2.1 + r.uniform(0, 1)
        root = C.add("cone", vertices=6, radius1=0.09, radius2=0.02, depth=0.32,
                     location=(math.cos(ang) * 0.18, math.sin(ang) * 0.18, 0.06))
        root.rotation_euler = (math.sin(ang) * 1.35, -math.cos(ang) * 1.35, 0)
        C.finish(root, m["bark"])
    for i in range(14):
        ang = r.uniform(0, math.tau)
        d = r.uniform(0.2, 0.8) if i else 0
        z = 2.05 + r.uniform(-0.35, 0.5) - d * 0.35
        s = C.add("ico_sphere", subdivisions=3, radius=r.uniform(0.42, 0.62),
                  location=(math.cos(ang) * d, math.sin(ang) * d * 0.7, z))
        C.displace(s, 0.2, 0.22, r.random() * 99)
        C.finish(s, m["leaf"])


def pine(r, m):
    trunk = C.add("cylinder", vertices=8, radius=0.1, depth=0.9, location=(0, 0, 0.45))
    C.finish(trunk, m["bark"])
    z = 0.55
    for i, (rad, h) in enumerate([(0.85, 1.0), (0.68, 0.95), (0.5, 0.85), (0.3, 0.8)]):
        rad *= r.uniform(0.9, 1.1)
        cone = C.add("cone", vertices=14, radius1=rad, radius2=0.0, depth=h, location=(0, 0, z + h / 2))
        C.displace(cone, 0.12, 0.12, r.random() * 99)
        C.finish(cone, m["pine"], smooth=False)
        z += h * 0.6


def dead_tree(r, m):
    trunk = C.add("cone", vertices=8, radius1=0.2, radius2=0.08, depth=2.0, location=(0, 0, 1.0))
    C.finish(C.displace(trunk, 0.05, 0.2, r.random() * 99), m["bark"])
    for i in range(6):
        z = r.uniform(0.9, 1.9)
        ang = r.uniform(0, math.tau)
        L = r.uniform(0.5, 0.9)
        b = C.add("cone", vertices=6, radius1=0.06, radius2=0.01, depth=L)
        b.location = (math.cos(ang) * L * 0.4, math.sin(ang) * L * 0.4, z + L * 0.3)
        b.rotation_euler = (math.sin(ang) * 0.9, -math.cos(ang) * 0.9, 0)
        C.finish(b, m["bark"])


def rock(r, m, size=0.38):
    s = C.add("ico_sphere", subdivisions=3, radius=size, location=(0, 0, size * 0.55))
    s.scale = (r.uniform(1.0, 1.25), r.uniform(0.85, 1.1), r.uniform(0.7, 0.9))
    C.displace(s, size * 0.35, 0.35, r.random() * 99)
    C.finish(s, m["stone"])
    return s


def boulder(r, m):
    rock(r, m, 0.8)
    moss = C.add("ico_sphere", subdivisions=3, radius=0.62, location=(0.05, 0, 0.95))
    moss.scale = (1.1, 0.9, 0.35)
    C.displace(moss, 0.2, 0.2, r.random() * 99)
    C.finish(moss, m["moss"])
    rock(r, m, 0.3).location = (0.75, -0.3, 0.15)


def bush(r, m):
    for i in range(4):
        ang = i * 1.6 + r.uniform(0, 1)
        s = C.add("ico_sphere", subdivisions=3, radius=r.uniform(0.24, 0.32),
                  location=(math.cos(ang) * 0.16, math.sin(ang) * 0.12, 0.27 + r.uniform(0, 0.1)))
        C.displace(s, 0.12, 0.12, r.random() * 99)
        C.finish(s, m["leaf"] if i % 2 else m["leaf_dark"])
    berry = C.material("berry", (0.85, 0.08, 0.1), rough=0.3) if r.random() < 0.6 else None
    if berry:
        for i in range(7):
            ang = r.uniform(0, math.tau)
            b = C.add("uv_sphere", radius=0.045, location=(math.cos(ang) * 0.3, math.sin(ang) * 0.22 - 0.08, r.uniform(0.25, 0.5)))
            C.finish(b, berry)


def flowers(r, m):
    colors = [(0.95, 0.85, 0.2), (0.95, 0.3, 0.45), (0.7, 0.45, 0.95), (1, 1, 1)]
    petal = C.material("petal", colors[r.randrange(len(colors))], rough=0.5)
    for i in range(9):
        x, y = r.uniform(-0.35, 0.35), r.uniform(-0.3, 0.3)
        h = r.uniform(0.12, 0.3)
        stem = C.add("cylinder", vertices=5, radius=0.015, depth=h, location=(x, y, h / 2))
        C.finish(stem, m["reed"])
        f = C.add("uv_sphere", radius=0.06, location=(x, y, h))
        f.scale.z = 0.5
        C.finish(f, petal)
    for i in range(10):
        g = C.add("cone", vertices=4, radius1=0.03, radius2=0, depth=r.uniform(0.1, 0.22),
                  location=(r.uniform(-0.4, 0.4), r.uniform(-0.35, 0.35), 0.06))
        g.rotation_euler = (r.uniform(-0.4, 0.4), r.uniform(-0.4, 0.4), 0)
        C.finish(g, m["leaf"])


def cactus(r, m):
    body = C.add("cylinder", vertices=12, radius=0.2, depth=1.3, location=(0, 0, 0.65))
    C.finish(body, m["cactus"], bevel=0.12, segments=3)
    top = C.add("uv_sphere", radius=0.2, location=(0, 0, 1.3))
    C.finish(top, m["cactus"])
    for side in (-1, 1):
        h = r.uniform(0.35, 0.6)
        arm = C.add("cylinder", vertices=10, radius=0.11, depth=0.35, location=(side * 0.3, 0, h))
        arm.rotation_euler.y = math.pi / 2
        C.finish(arm, m["cactus"])
        up = C.add("cylinder", vertices=10, radius=0.11, depth=0.45, location=(side * 0.45, 0, h + 0.22))
        C.finish(up, m["cactus"])
        cap = C.add("uv_sphere", radius=0.11, location=(side * 0.45, 0, h + 0.45))
        C.finish(cap, m["cactus"])


def reeds(r, m):
    for i in range(12):
        x, y = r.uniform(-0.35, 0.35), r.uniform(-0.25, 0.25)
        h = r.uniform(0.6, 1.3)
        s = C.add("cone", vertices=4, radius1=0.035, radius2=0.005, depth=h, location=(x, y, h / 2))
        s.rotation_euler = (r.uniform(-0.2, 0.2), r.uniform(-0.2, 0.2), 0)
        C.finish(s, m["reed"])
        if r.random() < 0.4:
            head = C.add("cylinder", vertices=6, radius=0.04, depth=0.18, location=(x, y, h * 0.85))
            C.finish(head, m["wood"])


def dungeon_entrance(r, m):
    for side in (-1, 1):
        pillar = C.add("cube", size=1, location=(side * 1.05, 0, 1.0))
        pillar.scale = (0.45, 0.6, 2.0)
        C.finish(C.displace(pillar, 0.05, 0.3, side + 5), m["stone"], bevel=0.05)
    lintel = C.add("cube", size=1, location=(0, 0, 2.15))
    lintel.scale = (2.7, 0.7, 0.45)
    C.finish(lintel, m["stone_dark"], bevel=0.06)
    cap = C.add("cube", size=1, location=(0, 0, 2.5))
    cap.scale = (1.2, 0.5, 0.3)
    C.finish(cap, m["stone"], bevel=0.05)
    hole = C.add("cube", size=1, location=(0, 0.25, 0.95))
    hole.scale = (1.7, 0.2, 1.9)
    C.finish(hole, m["dark"])
    for i in range(4):  # steps going down
        step = C.add("cube", size=1, location=(0, -0.15 + i * 0.12, 0.12 - i * 0.1))
        step.scale = (1.6, 0.25, 0.1)
        C.finish(step, m["stone_dark"])
    for side in (-1, 1):  # braziers
        bowl = C.add("cylinder", vertices=10, radius=0.16, depth=0.12, location=(side * 1.05, -0.35, 2.05 - 1.1))
        C.finish(bowl, m["iron"])
    rock(r, m, 0.25).location = (1.35, -0.5, 0.1)


def stairs_down(r, m):
    frame = C.add("cube", size=1, location=(0, 0, 0.04))
    frame.scale = (0.95, 0.95, 0.08)
    C.finish(frame, m["stone"], bevel=0.03)
    hole = C.add("cube", size=1, location=(0, 0, 0.05))
    hole.scale = (0.75, 0.75, 0.08)
    C.finish(hole, m["dark"])
    for i in range(4):
        s = C.add("cube", size=1, location=(0, -0.28 + i * 0.16, 0.06 - i * 0.08))
        s.scale = (0.7, 0.15, 0.06)
        C.finish(s, m["stone_dark"])


def dungeon_exit(r, m):
    ring = C.add("torus", major_radius=0.75, minor_radius=0.14, location=(0, 0, 1.0))
    ring.rotation_euler.x = math.pi / 2
    C.finish(C.displace(ring, 0.04, 0.2, 3), m["stone"])
    swirl = C.add("cylinder", vertices=32, radius=0.66, depth=0.05, location=(0, 0, 1.0))
    swirl.rotation_euler.x = math.pi / 2
    C.finish(swirl, m["magic"])
    base = C.add("cube", size=1, location=(0, 0, 0.12))
    base.scale = (1.5, 0.6, 0.24)
    C.finish(base, m["stone_dark"], bevel=0.04)
    for i in range(6):
        ang = i * math.tau / 6
        g = C.add("ico_sphere", subdivisions=1, radius=0.07, location=(math.cos(ang) * 0.95, -0.1, 1.0 + math.sin(ang) * 0.95))
        C.finish(g, m["magic"], smooth=False)


def chest(r, m):
    body = C.add("cube", size=1, location=(0, 0, 0.2))
    body.scale = (0.7, 0.45, 0.4)
    C.finish(body, m["wood"], bevel=0.03)
    lid = C.add("cylinder", vertices=16, radius=0.225, depth=0.7, location=(0, 0, 0.4))
    lid.rotation_euler.y = math.pi / 2
    lid.scale.x = 0.75
    C.finish(lid, m["wood"])
    for x in (-0.28, 0.28):
        band = C.add("cube", size=1, location=(x, 0, 0.3))
        band.scale = (0.07, 0.47, 0.62)
        C.finish(band, m["iron"], bevel=0.01)
    lock = C.add("cube", size=1, location=(0, -0.24, 0.33))
    lock.scale = (0.12, 0.04, 0.14)
    C.finish(lock, m["gold"], bevel=0.01)


def torch(r, m):
    pole = C.add("cylinder", vertices=8, radius=0.05, depth=1.1, location=(0, 0, 0.55))
    C.finish(pole, m["wood"])
    bowl = C.add("cone", vertices=10, radius1=0.08, radius2=0.16, depth=0.15, location=(0, 0, 1.15))
    C.finish(bowl, m["iron"])
    flame = C.add("cone", vertices=10, radius1=0.17, radius2=0.0, depth=0.55, location=(0, 0, 1.47))
    C.finish(C.displace(flame, 0.04, 0.1, 1), m["fire"])
    core = C.add("uv_sphere", radius=0.09, location=(0, 0, 1.33))
    C.finish(core, C.material("core", (1, 0.85, 0.3), emit=(1, 0.8, 0.3), strength=4))
    base = C.add("cylinder", vertices=8, radius=0.14, depth=0.08, location=(0, 0, 0.04))
    C.finish(base, m["stone"])


def anvil(r, m):
    block = C.add("cube", size=1, location=(0, 0, 0.14))
    block.scale = (0.5, 0.36, 0.28)
    C.finish(block, m["wood"], bevel=0.02)
    foot = C.add("cube", size=1, location=(0, 0, 0.35))
    foot.scale = (0.3, 0.2, 0.16)
    C.finish(foot, m["iron"], bevel=0.02)
    top = C.add("cube", size=1, location=(-0.04, 0, 0.48))
    top.scale = (0.62, 0.26, 0.14)
    C.finish(top, m["iron"], bevel=0.03)
    horn = C.add("cone", vertices=12, radius1=0.12, radius2=0.01, depth=0.3, location=(0.4, 0, 0.5))
    horn.rotation_euler.y = math.pi / 2
    C.finish(horn, m["iron"])
    ham = C.add("cube", size=1, location=(-0.1, -0.05, 0.6))
    ham.scale = (0.12, 0.08, 0.08)
    C.finish(ham, m["iron"], bevel=0.01)


def shrine(r, m):
    for i, (w, h, z) in enumerate([(1.3, 0.25, 0.12), (0.9, 0.3, 0.4), (0.55, 0.6, 0.85)]):
        s = C.add("cube", size=1, location=(0, 0, z))
        s.scale = (w, w * 0.7, h)
        C.finish(s, m["stone"] if i != 1 else m["stone_dark"], bevel=0.04)
    crystal = C.add("cone", vertices=6, radius1=0.25, radius2=0.0, depth=0.6, location=(0, 0, 1.75))
    C.finish(crystal, m["magic"], smooth=False)
    low = C.add("cone", vertices=6, radius1=0.25, radius2=0.0, depth=0.35, location=(0, 0, 1.3))
    low.rotation_euler.x = math.pi
    C.finish(low, m["magic"], smooth=False)
    for i in range(3):
        ang = i * math.tau / 3
        o = C.add("ico_sphere", subdivisions=1, radius=0.06, location=(math.cos(ang) * 0.5, math.sin(ang) * 0.3, 1.5))
        C.finish(o, m["magic"], smooth=False)


def bones(r, m):
    skull = C.add("uv_sphere", radius=0.14, location=(r.uniform(-0.1, 0.1), 0, 0.12))
    skull.scale = (1, 1.15, 0.9)
    C.finish(skull, m["bone"])
    for side in (-1, 1):
        eye = C.add("uv_sphere", radius=0.035, location=(skull.location.x + side * 0.05, -0.14, 0.14))
        C.finish(eye, m["dark"])
    for i in range(3):
        b = C.add("cylinder", vertices=6, radius=0.025, depth=r.uniform(0.3, 0.45),
                  location=(r.uniform(-0.3, 0.3), r.uniform(-0.2, 0.25), 0.03))
        b.rotation_euler = (math.pi / 2, 0, r.uniform(0, math.pi))
        C.finish(b, m["bone"])


MODELS = {name: globals()[name] for name, *_ in PROPS.values()}


def screen_bounds():
    """Bounds of every (modified) vertex in camera screen units (1 = 1 tile),
    with the prop's base point at (0, 0)."""
    e = math.radians(ELEVATION)
    up = Vector((0, math.sin(e), math.cos(e)))
    dg = bpy.context.evaluated_depsgraph_get()
    xs, ys = [], []
    for o in C.meshes():
        ev = o.evaluated_get(dg)
        me = ev.to_mesh()
        for v in me.vertices:
            p = ev.matrix_world @ v.co
            xs.append(p.x)
            ys.append(p.dot(up))
        ev.to_mesh_clear()
    return min(xs), max(xs), min(ys), max(ys)


def render_prop(obj_id, variant, out_dir):
    """Renders a prop on a canvas sized to fit it in whole tiles. Returns
    the sprite size and which tile of it is the prop's own cell."""
    name = PROPS[obj_id][0]
    r = random.Random(obj_id * 1000 + variant)
    C.reset(64, 64, samples=32)
    MODELS[name](r, mats(r))
    C.transform_all(Matrix.Scale(WORLD_SCALE, 4))
    pad = 0.03
    x0, x1, y0, y1 = screen_bounds()
    left = max(0, math.ceil(-x0 + pad - 0.5))
    right = max(0, math.ceil(x1 + pad - 0.5))
    above = max(0, math.ceil(y1 + pad - 0.5))
    below = max(0, math.ceil(-y0 + pad - 0.5))
    # keep the base cell in the middle column so the sprite is symmetric
    left = right = max(left, right)
    w, h = left + 1 + right, above + 1 + below
    sc = bpy.context.scene
    sc.render.resolution_x = w * TILE_PX * SUPERSAMPLE
    sc.render.resolution_y = h * TILE_PX * SUPERSAMPLE
    C.sun_rig()
    cam = C.ortho_camera(max(w, h), ELEVATION, shift_up=(above - below) / 2)
    path = Path(out_dir) / f"{name}_{variant}.png"
    C.render(path)
    return {"size": [w, h], "base": [left, above]}


def main():
    out = Path(sys.argv[-1] if len(sys.argv) > 1 else "build/art")
    raw = out / "raw_props"
    raw.mkdir(parents=True, exist_ok=True)
    only = set(a for a in sys.argv[1:-1])
    meta = {}
    for obj_id, (name, _w, _h, variants) in PROPS.items():
        if only and name not in only:
            continue
        frames = []
        for v in range(variants):
            f = render_prop(obj_id, v, raw)
            print("render", name, v, f, flush=True)
            frames.append(f)
        meta[obj_id] = {"name": name, "variants": frames}
    (raw / "props.json").write_text(json.dumps(meta, indent=1))


if __name__ == "__main__":
    main()
