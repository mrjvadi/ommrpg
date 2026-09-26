"""3D UI icons for OMMRPG, modelled in code and rendered with Blender.

Every icon shares one studio rig (key light from the upper left, cool
fill, warm rim) and one camera, then gets a dark outline and a soft drop
shadow (see compose()), the way mobile RPGs make their icons readable on
any background. Names match the client's icon names (UiIcon).

    python3 tools/blender/icons.py build/art [name ...]
"""
import math
import random
import sys
from pathlib import Path

import bpy  # noqa: F401  (loads mathutils)
from mathutils import Matrix, Vector
from PIL import Image, ImageFilter

sys.path.insert(0, str(Path(__file__).parent))
import common as C  # noqa: E402

RES = 320
OUT_SIZE = 192

# ---------------------------------------------------------------- materials

def M():
    return {
        "steel": C.material("steel", (0.78, 0.82, 0.9), metal=1.0, rough=0.22),
        "dark_steel": C.material("dsteel", (0.35, 0.37, 0.42), metal=1.0, rough=0.35),
        "gold": C.material("gold", (1.0, 0.68, 0.22), metal=1.0, rough=0.22),
        "leather": C.material("leather", (0.45, 0.2, 0.08), rough=0.55, noise=0.2, noise_scale=20),
        "leather_dark": C.material("leather_dark", (0.25, 0.1, 0.05), rough=0.6),
        "wood": C.material("wood", (0.55, 0.28, 0.1), rough=0.5, noise=0.25, noise_scale=14),
        "parchment": C.material("parch", (0.93, 0.83, 0.62), rough=0.8, noise=0.12, noise_scale=6),
        "red": C.material("red", (0.85, 0.08, 0.06), rough=0.35, coat=0.5),
        "blue": C.material("blue", (0.1, 0.35, 0.85), rough=0.35, coat=0.5),
        "white": C.material("white", (0.95, 0.93, 0.9), rough=0.4),
        "stone": C.material("stone", (0.5, 0.48, 0.46), rough=0.85, noise=0.3, noise_scale=8),
        "ruby": C.material("ruby", (1.0, 0.08, 0.1), rough=0.05, emit=(1.0, 0.1, 0.1), strength=0.6, coat=1),
        "sapphire": C.material("sapph", (0.1, 0.45, 1.0), rough=0.05, emit=(0.1, 0.45, 1.0), strength=0.8, coat=1),
        "amethyst": C.material("amet", (0.6, 0.2, 1.0), rough=0.08, emit=(0.55, 0.2, 1.0), strength=0.7, coat=1),
        "emerald": C.material("emer", (0.1, 0.85, 0.35), rough=0.08, emit=(0.1, 0.8, 0.3), strength=0.6, coat=1),
        "glass": C.material("glass", (0.95, 0.97, 1.0), rough=0.03, transmission=1.0),
        "bone": C.material("bone", (0.92, 0.88, 0.76), rough=0.5),
        "dark": C.material("dark", (0.03, 0.02, 0.02), rough=1.0),
        "green": C.material("green", (0.2, 0.6, 0.2), rough=0.4, coat=0.4),
    }


def box(size, loc, mat, bevel=0.04, rot=None):
    o = C.add("cube", size=1, location=loc)
    o.scale = size
    if rot:
        o.rotation_euler = rot
    return C.finish(o, mat, bevel=bevel, segments=3)


def cyl(r, depth, loc, mat, rot=None, verts=32, bevel=0.0):
    o = C.add("cylinder", vertices=verts, radius=r, depth=depth, location=loc)
    if rot:
        o.rotation_euler = rot
    return C.finish(o, mat, bevel=bevel, segments=3)


def sphere(r, loc, mat, scale=None):
    o = C.add("uv_sphere", radius=r, location=loc, segments=32, ring_count=16)
    if scale:
        o.scale = scale
    return C.finish(o, mat)


def torus(R, r, loc, mat, rot=None):
    o = C.add("torus", major_radius=R, minor_radius=r, location=loc)
    if rot:
        o.rotation_euler = rot
    return C.finish(o, mat)


def extrude(points, depth, loc, mat, rot=(math.pi / 2, 0, 0), bevel=0.03):
    """A flat polygon (x, z) extruded along y: stars, arrows, shields."""
    n = len(points)
    verts = [(x, -depth / 2, z) for x, z in points] + [(x, depth / 2, z) for x, z in points]
    faces = [list(range(n))[::-1], list(range(n, 2 * n))]
    for i in range(n):
        j = (i + 1) % n
        faces.append([i, j, n + j, n + i])
    me = bpy.data.meshes.new("ex")
    me.from_pydata(verts, [], faces)
    me.update()
    o = bpy.data.objects.new("ex", me)
    bpy.context.collection.objects.link(o)
    o.location = loc
    return C.finish(o, mat, bevel=bevel, segments=2, smooth=False)


def tilt(deg, axis='Y'):
    C.transform_all(Matrix.Rotation(math.radians(deg), 4, axis))

# ---------------------------------------------------------------- models (≈ 2 units tall)

def sword(m, blade="steel"):
    L, W, T = 1.9, 0.2, 0.05
    stations = [(0.0, 1.0), (0.74, 1.0), (0.9, 0.55), (1.0, 0.0)]
    verts, faces = [], []
    for z, k in stations:
        verts += [(-W * k, 0, z * L), (0, -T * k - 0.004, z * L), (W * k, 0, z * L), (0, T * k + 0.004, z * L)]
    for i in range(len(stations) - 1):
        a, b = i * 4, (i + 1) * 4
        for j in range(4):
            faces.append((a + j, a + (j + 1) % 4, b + (j + 1) % 4, b + j))
    faces.append((3, 2, 1, 0))
    me = bpy.data.meshes.new("blade")
    me.from_pydata(verts, [], faces)
    me.update()
    o = bpy.data.objects.new("blade", me)
    bpy.context.collection.objects.link(o)
    C.finish(o, m[blade], smooth=False)
    box((0.85, 0.16, 0.12), (0, 0, -0.05), m["gold"], 0.05)
    for s in (-1, 1):
        sphere(0.1, (s * 0.45, 0, -0.05), m["gold"])
    cyl(0.075, 0.55, (0, 0, -0.4), m["leather"])
    for z in (-0.25, -0.4, -0.55):
        torus(0.08, 0.018, (0, 0, z), m["gold"])
    sphere(0.13, (0, 0, -0.75), m["gold"])
    sphere(0.07, (0, -0.12, -0.05), m["ruby"])
    C.transform_all(Matrix.Translation((0, 0, -0.45)))
    tilt(-42)


def attack(m):
    sword(m)
    C.transform_all(Matrix.Translation((0.1, 0, 0)))
    for o in list(C.meshes()):
        c = o.copy()
        c.data = o.data.copy()
        bpy.context.collection.objects.link(c)
        c.matrix_world = Matrix.Scale(-1, 4, (1, 0, 0)) @ o.matrix_world
        c.location.y += 0.3


def helmet(m):
    dome = sphere(0.7, (0, 0, 0.15), m["steel"], scale=(1, 1.05, 1.1))
    cyl(0.72, 0.5, (0, 0, -0.3), m["steel"])
    box((0.9, 0.12, 0.06), (0, -0.7, 0.05), m["dark"], 0.02)       # visor slit
    box((0.08, 0.12, 0.7), (0, -0.7, -0.15), m["dark"], 0.02)
    box((0.12, 0.1, 1.4), (0, -0.05, 0.3), m["gold"], 0.04)        # crest ridge
    torus(0.72, 0.05, (0, 0, -0.52), m["gold"])
    plume = sphere(0.3, (0, 0.2, 1.05), m["red"], scale=(0.5, 1.6, 0.8))
    plume.rotation_euler.x = 0.4
    for s in (-1, 1):
        sphere(0.06, (s * 0.72, -0.1, -0.3), m["gold"])


def bag(m):
    box((1.15, 0.6, 1.35), (0, 0, -0.1), m["leather"], 0.25)
    flap = box((1.2, 0.2, 0.7), (0, -0.28, 0.35), m["leather_dark"], 0.15)
    flap.rotation_euler.x = -0.12
    box((0.62, 0.22, 0.5), (0, -0.36, -0.45), m["leather_dark"], 0.1)   # pocket
    box((0.22, 0.05, 0.16), (0, -0.44, 0.1), m["gold"], 0.02)           # buckle
    for s in (-1, 1):
        box((0.12, 0.05, 1.2), (s * 0.35, -0.33, 0.0), m["leather_dark"], 0.02)
    torus(0.28, 0.06, (0, 0.05, 0.75), m["leather_dark"], rot=(math.pi / 2, 0, 0))
    sphere(0.2, (0.5, -0.1, 0.9), m["parchment"], scale=(0.4, 0.4, 1.4))  # rolled map sticking out


def trade(m):
    box((1.8, 0.8, 0.7), (0, 0, -0.55), m["wood"], 0.05)       # counter
    box((1.9, 0.9, 0.08), (0, 0, -0.18), m["wood"], 0.02)
    for x in (-0.85, 0.85):
        cyl(0.05, 1.5, (x, -0.35, 0.3), m["wood"])
    for i in range(6):  # striped awning
        mat = m["red"] if i % 2 == 0 else m["white"]
        seg = box((0.33, 1.0, 0.1), (-0.83 + i * 0.333, -0.1, 1.05), mat, 0.02)
        seg.rotation_euler.x = 0.35
        sphere(0.17, (-0.83 + i * 0.333, -0.6, 0.86), mat, scale=(1, 0.4, 0.6))
    sphere(0.22, (-0.4, -0.1, 0.05), m["red"])        # goods: apple, potion, coins
    cyl(0.13, 0.3, (0.1, -0.05, 0.02), m["sapphire"])
    for i in range(4):
        cyl(0.16, 0.05, (0.55, -0.05, -0.1 + i * 0.055), m["gold"])


def scroll(m):
    sheet = box((1.3, 0.04, 1.2), (0, 0, 0), m["parchment"], 0.02)
    for z in (0.62, -0.62):
        cyl(0.14, 1.45, (0, 0, z), m["parchment"], rot=(0, math.pi / 2, 0))
        for x in (-0.78, 0.78):
            sphere(0.1, (x, 0, z), m["wood"])
    for i in range(5):
        box((0.9 - (i % 2) * 0.25, 0.02, 0.05), (-0.05, -0.03, 0.35 - i * 0.18), m["dark"], 0.0)
    torus(0.18, 0.04, (0.35, -0.06, -0.35), m["red"], rot=(math.pi / 2, 0, 0))
    tilt(-12)


def book(m):
    box((1.2, 0.35, 1.5), (0, 0, 0), m["red"], 0.06)
    box((1.1, 0.3, 1.42), (0.06, 0.0, 0), m["parchment"], 0.02)
    box((1.22, 0.07, 1.52), (0, -0.16, 0), m["red"], 0.05)
    box((1.22, 0.07, 1.52), (0, 0.16, 0), m["red"], 0.05)
    for x, z in ((-0.55, 0.7), (0.55, 0.7), (-0.55, -0.7), (0.55, -0.7)):
        box((0.18, 0.1, 0.18), (x, -0.2, z), m["gold"], 0.03)
    sphere(0.16, (0, -0.22, 0.05), m["emerald"], scale=(1, 0.5, 1.2))
    tilt(-15, 'Z')


def gear(m):
    cyl(0.7, 0.3, (0, 0, 0), m["dark_steel"], rot=(math.pi / 2, 0, 0), bevel=0.03)
    for i in range(10):
        a = i * math.tau / 10
        t = box((0.28, 0.3, 0.26), (math.cos(a) * 0.82, 0, math.sin(a) * 0.82), m["dark_steel"], 0.04)
        t.rotation_euler.y = -a
    cyl(0.28, 0.34, (0, 0, 0), m["gold"], rot=(math.pi / 2, 0, 0), bevel=0.03)
    cyl(0.14, 0.4, (0, 0, 0), m["dark"], rot=(math.pi / 2, 0, 0))


def layout(m):
    arrow = [(-0.12, 0.0), (0.12, 0.0), (0.12, 0.55), (0.3, 0.55), (0.0, 0.95), (-0.3, 0.55), (-0.12, 0.55)]
    for i in range(4):
        a = extrude(arrow, 0.2, (0, 0, 0), m["gold"])
        a.rotation_euler = (math.pi / 2, i * math.pi / 2, 0)
        a.rotation_euler = (0, i * math.pi / 2, 0)
    sphere(0.25, (0, -0.1, 0), m["sapphire"])


def exit(m):
    door = [(-0.55, -0.9)] + [(math.cos(a) * 0.55, 0.3 + math.sin(a) * 0.55) for a in [i * math.pi / 12 for i in range(13)]] + [(0.55, -0.9)]
    door = [door[0]] + door[1:-1][::-1] + [door[-1]]
    frame = [(x * 1.25, z * 1.12 - 0.05) for x, z in door]
    extrude(frame, 0.2, (0, 0.08, 0), m["stone"], bevel=0.04)
    extrude(door, 0.24, (0, -0.02, 0), m["wood"], bevel=0.03)
    for x in (-0.18, 0.18):
        box((0.04, 0.05, 1.5), (x, -0.16, -0.05), m["leather_dark"], 0.0)
    for z in (-0.5, 0.3):
        box((1.0, 0.05, 0.1), (0, -0.16, z), m["dark_steel"], 0.01)
    sphere(0.08, (0.35, -0.2, -0.2), m["gold"])


def hand(m, mat="leather"):
    box((0.7, 0.3, 0.75), (0, 0, -0.1), m[mat], 0.15)                 # palm
    for i, (x, h) in enumerate([(-0.25, 0.55), (-0.08, 0.65), (0.09, 0.62), (0.26, 0.5)]):
        cyl(0.085, h, (x, 0, 0.27 + h / 2), m[mat], bevel=0.02)
        sphere(0.085, (x, 0, 0.27 + h), m[mat])
    thumb = cyl(0.1, 0.5, (-0.45, -0.05, 0.05), m[mat], bevel=0.02)
    thumb.rotation_euler.y = 0.9
    cyl(0.42, 0.35, (0, 0, -0.62), m[mat if mat != "steel" else "dark_steel"], bevel=0.04)  # cuff
    torus(0.42, 0.05, (0, 0, -0.47), m["gold"])
    tilt(-10)


def use(m):
    hand(m)


def hands(m):
    hand(m, "steel")


def coins(m, n=7):
    r = random.Random(4)
    for i in range(n):
        cyl(0.36, 0.1, (r.uniform(-0.03, 0.03) - 0.3, r.uniform(-0.03, 0.03), -0.7 + i * 0.1), m["gold"], bevel=0.02)
    for i in range(4):
        cyl(0.36, 0.1, (0.35 + r.uniform(-0.03, 0.03), 0.1, -0.7 + i * 0.1), m["gold"], bevel=0.02)
    c = cyl(0.42, 0.1, (0.15, -0.35, 0.05), m["gold"], rot=(math.pi / 2 - 0.25, 0, 0.35), bevel=0.02)
    cyl(0.22, 0.12, c.location, m["gold"], rot=c.rotation_euler, bevel=0.02)


def gold(m):
    coins(m)


def crystals(m, mat):
    for x, h, rad, t in [(0, 1.5, 0.3, 0), (-0.42, 0.95, 0.22, 24), (0.44, 1.05, 0.24, -22), (-0.2, 0.6, 0.15, 42), (0.22, 0.55, 0.14, -40)]:
        body = C.add("cylinder", vertices=6, radius=rad, depth=h, location=(0, 0, h / 2))
        tip = C.add("cone", vertices=6, radius1=rad, radius2=0.0, depth=rad * 1.7, location=(0, 0, h + rad * 0.85))
        for o in (body, tip):
            C.finish(o, m[mat], smooth=False)
            o.matrix_world = Matrix.Translation((x, 0, -0.75)) @ Matrix.Rotation(math.radians(t), 4, 'Y') @ o.matrix_world
    stone = C.add("ico_sphere", subdivisions=2, radius=0.55, location=(0, 0, -0.85))
    stone.scale = (1.4, 0.8, 0.4)
    C.finish(stone, m["stone"])


def essence(m):
    crystals(m, "amethyst")


def ton(m):
    # brilliant-cut gem: crown (frustum) + pavilion (cone)
    crown = C.add("cone", vertices=10, radius1=0.9, radius2=0.5, depth=0.35, location=(0, 0, 0.35))
    pav = C.add("cone", vertices=10, radius1=0.9, radius2=0.0, depth=1.0, location=(0, 0, -0.33))
    pav.rotation_euler.x = math.pi
    for o in (crown, pav):
        C.finish(o, m["sapphire"], smooth=False)
    tilt(12, 'X')


def wallet(m):
    sphere(0.75, (0, 0, -0.2), m["leather"], scale=(1, 0.8, 0.85))
    cyl(0.3, 0.35, (0, 0, 0.55), m["leather"], bevel=0.05)
    torus(0.3, 0.06, (0, 0, 0.45), m["gold"])
    for i in range(3):
        cyl(0.2, 0.06, (-0.2 + i * 0.22, -0.2, 0.75 + i * 0.03), m["gold"], rot=(0.9, 0.3 * i, 0))


def shield(m):
    pts = [(math.cos(a) * 0.85, math.sin(a) * 0.95) for a in [i * math.tau / 40 for i in range(40)]]
    shape = [(x, z if z > 0 else z * 1.25) for x, z in pts]
    extrude(shape, 0.18, (0, 0, 0), m["blue"], bevel=0.05)
    extrude([(x * 1.08, z * 1.08) for x, z in shape], 0.12, (0, 0.04, 0), m["gold"], bevel=0.04)
    sphere(0.22, (0, -0.12, 0.05), m["gold"], scale=(1, 0.6, 1))
    box((0.14, 0.05, 1.5), (0, -0.1, -0.05), m["gold"], 0.02)
    box((1.3, 0.05, 0.14), (0, -0.1, 0.2), m["gold"], 0.02)


def offhand(m):
    shield(m)


def hero(m):
    helmet(m)


def head(m):
    helmet(m)


def weapon(m):
    sword(m)


def chest(m):
    # cuirass: a rounded plate with a raised centre ridge, pauldrons and a belt
    plate = box((1.2, 0.55, 1.3), (0, 0, 0), m["steel"], 0.28)
    box((0.12, 0.2, 1.05), (0, -0.28, 0.02), m["gold"], 0.04)
    neck = [(-0.32, 0.66), (0.32, 0.66), (0.0, 0.3)]
    extrude(neck, 0.4, (0, -0.1, 0), m["dark"], bevel=0.0)
    for s_ in (-1, 1):
        pad = sphere(0.36, (s_ * 0.66, 0.02, 0.5), m["steel"], scale=(1.1, 0.9, 0.6))
        torus(0.34, 0.045, (s_ * 0.66, 0.02, 0.44), m["gold"], rot=(0, s_ * 0.5, 0))
    box((1.26, 0.6, 0.16), (0, 0, -0.55), m["leather"], 0.04)
    box((0.2, 0.1, 0.2), (0, -0.3, -0.55), m["gold"], 0.02)
    sphere(0.11, (0, -0.32, 0.15), m["ruby"], scale=(1, 0.5, 1))


def legs(m):
    for s in (-1, 1):
        cyl(0.26, 1.3, (s * 0.35, 0, 0), m["steel"], bevel=0.03)
        sphere(0.24, (s * 0.35, -0.12, 0.05), m["gold"], scale=(1, 0.6, 1))
        box((0.55, 0.7, 0.28), (s * 0.35, -0.18, -0.75), m["dark_steel"], 0.08)
    box((1.2, 0.5, 0.25), (0, 0, 0.75), m["leather"], 0.06)


def feet(m):
    cyl(0.3, 1.0, (0, 0.1, 0.2), m["leather"], bevel=0.05)
    box((0.62, 1.1, 0.4), (0, -0.25, -0.45), m["leather"], 0.18)
    torus(0.3, 0.06, (0, 0.1, 0.65), m["leather_dark"])
    box((0.64, 1.12, 0.1), (0, -0.25, -0.62), m["leather_dark"], 0.03)
    for z in (0.1, 0.35):
        box((0.12, 0.08, 0.1), (0, -0.2, z), m["gold"], 0.02)
    tilt(-75, 'Z')


def ring(m):
    torus(0.65, 0.13, (0, 0, -0.1), m["gold"], rot=(1.1, 0, 0))
    box((0.35, 0.35, 0.2), (0, -0.2, 0.55), m["gold"], 0.06)
    gem = C.add("cone", vertices=8, radius1=0.28, radius2=0.0, depth=0.35, location=(0, -0.2, 0.78))
    C.finish(gem, m["ruby"], smooth=False)


def amulet(m):
    pts = [(math.cos(a) * 0.9, 0.3 + math.sin(a) * 0.75) for a in [math.pi * 0.05 + i * math.pi * 0.9 / 16 for i in range(17)]]
    for x, z in pts:
        sphere(0.06, (x, 0, z), m["gold"])
    extrude([(0, -0.95), (0.45, -0.35), (0, 0.2), (-0.45, -0.35)], 0.2, (0, 0, 0), m["gold"], bevel=0.04)
    sphere(0.25, (0, -0.14, -0.38), m["emerald"], scale=(1, 0.5, 1.2))


def potion(m):
    sphere(0.62, (0, 0, -0.3), m["glass"])
    sphere(0.56, (0, 0, -0.32), m["red"], scale=(1, 1, 0.85))
    cyl(0.18, 0.45, (0, 0, 0.45), m["glass"])
    torus(0.2, 0.05, (0, 0, 0.62), m["gold"])
    cyl(0.15, 0.22, (0, 0, 0.76), m["wood"], bevel=0.02)


def vault(m):
    box((1.5, 1.0, 0.8), (0, 0, -0.3), m["wood"], 0.05)
    # open lid: a half barrel leaning back behind the chest
    lid_pts = [(math.cos(a) * 0.5, math.sin(a) * 0.5) for a in [i * math.pi / 12 for i in range(13)]]
    lid = extrude(lid_pts, 1.5, (0, 0, 0), m["wood"], rot=(0, 0, 0), bevel=0.03)
    lid.rotation_euler = (0, 0, math.pi / 2)
    bpy.context.view_layer.update()
    lid.matrix_world = Matrix.Translation((0, 0.62, 0.35)) @ Matrix.Rotation(-1.25, 4, 'X') @ lid.matrix_world
    for x in (-0.6, 0.6):
        band = extrude([(math.cos(a) * 0.52, math.sin(a) * 0.52) for a in [i * math.pi / 12 for i in range(13)]], 0.12, (0, 0, 0), m["dark_steel"], bevel=0.01)
        band.rotation_euler = (0, 0, math.pi / 2)
        bpy.context.view_layer.update()
        band.matrix_world = Matrix.Translation((x, 0.62, 0.35)) @ Matrix.Rotation(-1.25, 4, 'X') @ band.matrix_world
    for x in (-0.6, 0, 0.6):
        box((0.12, 1.04, 0.84), (x, 0, -0.3), m["dark_steel"], 0.02)
    box((0.26, 0.1, 0.3), (0, -0.52, -0.15), m["gold"], 0.03)
    r = random.Random(3)
    for i in range(14):
        cyl(0.12, 0.03, (r.uniform(-0.55, 0.55), r.uniform(-0.3, 0.3), 0.15 + r.uniform(0, 0.15)), m["gold"],
            rot=(r.uniform(-0.6, 0.6), r.uniform(-0.6, 0.6), 0))
    sphere(0.14, (0.25, -0.1, 0.3), m["ruby"])
    tilt(-20, 'Z')


def star(m):
    pts = []
    for i in range(10):
        a = math.pi / 2 + i * math.pi / 5
        rr = 0.95 if i % 2 == 0 else 0.42
        pts.append((math.cos(a) * rr, math.sin(a) * rr))
    extrude(pts[::-1], 0.3, (0, 0, 0), m["gold"], bevel=0.08)


def heart(m):
    for s in (-1, 1):
        sphere(0.45, (s * 0.35, 0, 0.25), m["red"], scale=(1, 0.65, 1))
    cone = C.add("cone", vertices=32, radius1=0.62, radius2=0.0, depth=1.0, location=(0, 0, -0.4))
    cone.rotation_euler.x = math.pi
    cone.scale.y = 0.65
    C.finish(cone, m["red"])


def fist(m):
    hand(m, "steel")


def sparkle(m):
    pts = []
    for i in range(8):
        a = i * math.pi / 4
        rr = 1.0 if i % 2 == 0 else 0.22
        pts.append((math.cos(a) * rr, math.sin(a) * rr))
    s = extrude(pts[::-1], 0.2, (0, 0, 0), C.material("spark", (1.0, 0.75, 0.25), metal=1.0, rough=0.2, emit=(1, 0.7, 0.2), strength=0.25), bevel=0.05)


def crown(m):
    cyl(0.7, 0.4, (0, 0, -0.3), m["gold"], bevel=0.03)
    for i in range(5):
        a = -math.pi / 2 + (i - 2) * 0.55
        x, y = math.cos(a) * 0.65, math.sin(a) * 0.65
        spike = C.add("cone", vertices=4, radius1=0.18, radius2=0.0, depth=0.55, location=(x, y, 0.15))
        C.finish(spike, m["gold"], smooth=False)
        sphere(0.07, (x, y, 0.45), m["gold"])
    for i, g in enumerate(["ruby", "sapphire", "emerald"]):
        a = -math.pi / 2 + (i - 1) * 0.6
        sphere(0.1, (math.cos(a) * 0.7, math.sin(a) * 0.7, -0.3), m[g], scale=(1, 0.6, 1))


def home(m):
    box((1.2, 0.9, 0.8), (0, 0, -0.4), m["parchment"], 0.03)
    roof = C.add("cone", vertices=4, radius1=1.05, radius2=0.0, depth=0.8, location=(0, 0, 0.4))
    roof.rotation_euler.z = math.pi / 4
    roof.scale.y = 0.75
    C.finish(roof, m["red"], smooth=False)
    box((0.3, 0.1, 0.5), (0, -0.46, -0.55), m["wood"], 0.02)
    box((0.25, 0.1, 0.25), (0.35, -0.46, -0.3), m["sapphire"], 0.02)
    box((0.18, 0.18, 0.45), (0.4, 0.1, 0.55), m["stone"], 0.02)


def skull(m):
    sphere(0.7, (0, 0, 0.15), m["bone"], scale=(1, 1.05, 0.95))
    box((0.8, 0.6, 0.4), (0, -0.2, -0.45), m["bone"], 0.15)
    for s in (-1, 1):
        sphere(0.19, (s * 0.28, -0.58, 0.05), m["dark"], scale=(1, 0.5, 1.1))
    sphere(0.08, (0, -0.68, -0.2), m["dark"], scale=(1, 0.5, 1.2))
    for i in range(5):
        box((0.1, 0.06, 0.14), (-0.24 + i * 0.12, -0.52, -0.55), m["white"], 0.02)


def map(m):
    for i, (x, rot) in enumerate([(-0.55, 0.25), (0, -0.25), (0.55, 0.25)]):
        p = box((0.58, 0.04, 1.3), (x, 0, 0), m["parchment"], 0.01)
        p.rotation_euler.z = rot
    for (x0, z0), (x1, z1) in [((-0.7, -0.4), (-0.1, 0.1)), ((-0.1, 0.1), (0.4, -0.1))]:
        pass
    for dx in (-0.12, 0.12):
        b = box((0.06, 0.03, 0.4), (0.45 + dx * 0, -0.1, 0.25), m["red"], 0.0)
        b.rotation_euler.y = 0.8 if dx > 0 else -0.8
    sphere(0.08, (-0.5, -0.1, -0.3), m["red"])


def guild(m):
    cyl(0.05, 2.0, (-0.55, 0, 0), m["wood"])
    sphere(0.1, (-0.55, 0, 1.02), m["gold"])
    flag = [(-0.5, 0.9), (0.75, 0.9), (0.75, -0.3), (0.12, 0.0), (-0.5, -0.3)]
    extrude(flag, 0.08, (0, 0, 0), m["red"], bevel=0.02)
    star_pts = []
    for i in range(10):
        a = math.pi / 2 + i * math.pi / 5
        rr = 0.26 if i % 2 == 0 else 0.11
        star_pts.append((0.12 + math.cos(a) * rr, 0.4 + math.sin(a) * rr))
    extrude(star_pts[::-1], 0.1, (0, -0.03, 0), m["gold"], bevel=0.01)


def dungeon(m):
    for s in (-1, 1):
        box((0.35, 0.5, 1.4), (s * 0.65, 0, -0.2), m["stone"], 0.05)
    arch = [(math.cos(a) * 0.95, 0.5 + math.sin(a) * 0.6) for a in [i * math.pi / 12 for i in range(13)]]
    inner = [(math.cos(a) * 0.5, 0.5 + math.sin(a) * 0.3) for a in [i * math.pi / 12 for i in range(13)]][::-1]
    extrude(arch + inner, 0.5, (0, 0, 0), m["stone"], bevel=0.04)
    box((1.0, 0.1, 1.45), (0, 0.15, -0.2), m["dark"], 0.0)
    for s in (-1, 1):
        sphere(0.12, (s * 0.65, -0.3, 0.7), C.material(f"f{s}", (1, 0.5, 0.1), emit=(1, 0.5, 0.1), strength=5))


def anvil(m):
    box((1.3, 0.55, 0.3), (-0.1, 0, 0.35), m["dark_steel"], 0.05)
    horn = C.add("cone", vertices=24, radius1=0.25, radius2=0.02, depth=0.7, location=(0.85, 0, 0.38))
    horn.rotation_euler.y = math.pi / 2
    C.finish(horn, m["dark_steel"])
    box((0.55, 0.4, 0.45), (-0.1, 0, -0.05), m["dark_steel"], 0.05)
    box((1.0, 0.7, 0.25), (-0.1, 0, -0.4), m["dark_steel"], 0.05)
    box((0.9, 0.9, 0.4), (-0.1, 0, -0.75), m["wood"], 0.05)


def hammer(m):
    cyl(0.07, 1.6, (0, 0, -0.2), m["wood"])
    box((0.8, 0.35, 0.35), (0, 0, 0.65), m["dark_steel"], 0.06)
    torus(0.09, 0.03, (0, 0, 0.42), m["gold"])
    tilt(35)


MODELS = [n for n in ["attack", "weapon", "hero", "head", "bag", "trade", "chronicle", "scroll", "book", "settings",
                      "layout", "exit", "use", "hands", "fist", "gold", "essence", "ton", "wallet", "offhand", "chest",
                      "legs", "feet", "ring", "amulet", "potion", "vault", "star", "heart", "sparkle", "crown", "home",
                      "skull", "map", "guild", "dungeon", "anvil", "hammer"]]
ALIASES = {"chronicle": scroll, "settings": gear}


def compose(src, dst):
    """Outline + soft drop shadow, then downscale to the delivery size."""
    im = Image.open(src).convert("RGBA")
    a = im.split()[3]
    out = Image.new("RGBA", im.size, (0, 0, 0, 0))
    shadow = Image.new("RGBA", im.size, (0, 0, 0, 0))
    sa = a.filter(ImageFilter.MaxFilter(9)).filter(ImageFilter.GaussianBlur(6)).point(lambda v: int(v * 0.55))
    shadow.putalpha(sa)
    out.alpha_composite(shadow, (0, 8))
    ol = Image.new("RGBA", im.size, (20, 11, 6, 255))
    ol.putalpha(a.filter(ImageFilter.MaxFilter(9)))
    out.alpha_composite(ol)
    out.alpha_composite(im)
    out.resize((OUT_SIZE, OUT_SIZE), Image.LANCZOS).save(dst, optimize=True)


def render_icon(name, raw_dir, out_dir):
    C.reset(RES, RES, samples=48)
    fn = ALIASES.get(name) or globals()[name]
    fn(M())
    C.studio_rig()
    C.icon_camera(2.75)
    raw = Path(raw_dir) / f"{name}.png"
    C.render(raw)
    compose(raw, Path(out_dir) / f"{name}.png")


def main():
    root = Path(sys.argv[1] if len(sys.argv) > 1 else "build/art")
    only = sys.argv[2:]
    raw = root / "raw_icons"
    out = root / "pack" / "icons"
    raw.mkdir(parents=True, exist_ok=True)
    out.mkdir(parents=True, exist_ok=True)
    for name in MODELS:
        if only and name not in only:
            continue
        print("icon", name, flush=True)
        render_icon(name, raw, out)


if __name__ == "__main__":
    main()
