"""GUI kit v2 for OMMRPG, modelled in code and rendered with Blender.

Art direction taken from classic MMO interfaces (Metin2, Aion, "The
Stone" kit, Chinese xianxia MMOs): heavy carved iron frames with an
engraved band, bronze corner brackets with jewels, recessed stone panels,
engraved metal buttons, deep item sockets, glass tube bars, an ornate
status plate (portrait orb + bars), a round minimap frame, orb frames for
actions and a crest on top of windows.

Materials are "weathered" (common.weathered): dirt in the crevices, worn
bright edges and a hammered surface. Everything is rendered at 2x and
packed into one atlas with 9-slice margins and anchor points (pack.py).
Engraved edge bands repeat every PERIOD px so the client can tile them.

    python3 tools/blender/gui.py build/art [part ...]
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

PX = 128.0          # rendered pixels per Blender unit (2x the on-screen size)
SAMPLES = 64
PERIOD = 32         # engraved pattern period along stretched edges (px)

# ---------------------------------------------------------------- scene

def scene(w_px, h_px):
    C.reset(int(w_px), int(h_px), samples=SAMPLES)
    sc = bpy.context.scene
    sc.view_settings.exposure = -0.25
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
    area((-5, -5, 5), 2000, (1.0, 0.86, 0.68), 3)    # warm key, upper left, a little raking
    area((6, -4, -3), 220, (0.45, 0.55, 0.75), 6)    # cool fill, lower right
    area((0, -3, 7), 260, (1.0, 0.9, 0.75), 8)       # top sheen
    C.world(sky=(0.55, 0.45, 0.33), ground=(0.02, 0.018, 0.015), strength=0.3)


def mats():
    return {
        "iron": C.weathered("iron", (0.1, 0.095, 0.092), rough=0.34, bump=0.14, dirt=0.8, wear=1.1),
        "iron_dark": C.weathered("iron_dark", (0.05, 0.047, 0.045), rough=0.5, bump=0.2, dirt=0.7, wear=0.8),
        "bronze": C.weathered("bronze", (0.55, 0.33, 0.14), rough=0.3, bump=0.18, dirt=0.85, wear=0.6),
        "gold": C.weathered("gold", (0.92, 0.66, 0.28), rough=0.22, bump=0.08, dirt=0.6, wear=0.35, hammered=False),
        "stone": C.carved_stone("stone", (0.014, 0.013, 0.012), bump=0.3),
        "stone_dark": C.carved_stone("stone_dark", (0.007, 0.0065, 0.006), bump=0.2),
        "stone_mid": C.carved_stone("stone_mid", (0.035, 0.032, 0.029), bump=0.3),
        "ruby": C.jewel("ruby", (0.85, 0.05, 0.04), 0.6),
        "sapphire": C.jewel("sapph", (0.08, 0.3, 0.9), 0.6),
        "emerald": C.jewel("emer", (0.05, 0.7, 0.3), 0.5),
        "amber": C.jewel("amber", (1.0, 0.55, 0.08), 0.6),
        "amethyst": C.jewel("amethyst", (0.5, 0.1, 0.9), 0.6),
    }


def u(px):
    return px / PX

# ---------------------------------------------------------------- geometry helpers (px, origin at the centre, z up)

def box(x0, z0, x1, z1, depth, mat, y=0.0, bevel=0.0, segments=3):
    o = C.add("cube", size=1, location=(u(x0 + x1) / 2, y, u(z0 + z1) / 2))
    o.scale = (u(abs(x1 - x0)), depth, u(abs(z1 - z0)))
    return C.finish(o, mat, bevel=bevel, segments=segments)


def frame(x0, z0, x1, z1, width, depth, mat, y=0.0, bevel=0.0):
    box(x0, z1 - width, x1, z1, depth, mat, y, bevel)
    box(x0, z0, x1, z0 + width, depth, mat, y, bevel)
    box(x0, z0 + width, x0 + width, z1 - width, depth, mat, y, bevel)
    box(x1 - width, z0 + width, x1, z1 - width, depth, mat, y, bevel)


def disc(x, z, r, depth, mat, y=0.0, bevel=0.0, verts=64):
    o = C.add("cylinder", vertices=verts, radius=u(r), depth=depth, location=(u(x), y, u(z)))
    o.rotation_euler = (math.pi / 2, 0, 0)
    return C.finish(o, mat, bevel=bevel)


def ring(x, z, r, thick, mat, y=0.0, squash=0.7):
    o = C.add("torus", major_radius=u(r), minor_radius=u(thick), location=(u(x), y, u(z)),
              major_segments=max(48, int(r)), minor_segments=16)
    o.rotation_euler = (math.pi / 2, 0, 0)
    o.scale = (1, 1, squash)
    return C.finish(o, mat)


def stud(x, z, r, mat, y=-0.05):
    o = C.add("uv_sphere", radius=u(r), location=(u(x), y, u(z)), segments=20, ring_count=10)
    o.scale = (1, 0.55, 1)
    return C.finish(o, mat)


def jewel(x, z, r, mat, setting, y=-0.08):
    g = C.add("uv_sphere", radius=u(r), location=(u(x), y, u(z)), segments=24, ring_count=12)
    g.scale = (1, 0.6, 1)
    C.finish(g, mat)
    ring(x, z, r * 1.08, max(1.5, r * 0.22), setting, y + 0.02, squash=1.0)
    for i in range(4):  # prongs
        a = i * math.pi / 2 + math.pi / 4
        stud(x + math.cos(a) * r * 1.05, z + math.sin(a) * r * 1.05, max(1.2, r * 0.2), setting, y - 0.02)


def diamond(x, z, r, depth, mat, y=-0.03, bevel=0.012):
    o = C.add("cube", size=1, location=(u(x), y, u(z)))
    o.scale = (u(r) * 1.41, depth, u(r) * 1.41)
    o.rotation_euler = (0, math.pi / 4, 0)
    return C.finish(o, mat, bevel=bevel, segments=3)


def spike(x, z, angle, length, r, mat, y=-0.04):
    """A cone pointing along `angle` (radians, 0 = +x, pi/2 = up)."""
    o = C.add("cone", vertices=12, radius1=u(r), radius2=0, depth=u(length),
              location=(u(x) + math.cos(angle) * u(length) / 2, y, u(z) + math.sin(angle) * u(length) / 2))
    o.rotation_euler = (0, math.pi / 2 - angle, 0)
    return C.finish(o, mat)


def fan(x, z, r, mat, y=0.0, depth=0.08, steps=40):
    """A half disc (flat side down), extruded towards the camera."""
    import bmesh
    me = bpy.data.meshes.new("fan")
    bm = bmesh.new()
    pts = [(u(x) + math.cos(math.pi * i / steps) * u(r), u(z) + math.sin(math.pi * i / steps) * u(r)) for i in range(steps + 1)]
    front = [bm.verts.new((px, y - depth / 2, pz)) for px, pz in pts]
    face = bm.faces.new(front)
    ext = bmesh.ops.extrude_face_region(bm, geom=[face])
    bmesh.ops.translate(bm, vec=(0, depth, 0), verts=[v for v in ext["geom"] if isinstance(v, bmesh.types.BMVert)])
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    bm.to_mesh(me)
    o = bpy.data.objects.new("fan", me)
    bpy.context.collection.objects.link(o)
    return C.finish(o, mat, bevel=0.01)


def _tube(name, thick, mat):
    cu = bpy.data.curves.new(name, 'CURVE')
    cu.dimensions = '3D'
    cu.bevel_depth = u(thick)
    cu.bevel_resolution = 3
    cu.use_fill_caps = True
    o = bpy.data.objects.new(name, cu)
    bpy.context.collection.objects.link(o)
    o.data.materials.append(mat)
    return cu, o


def curl(cx, cz, r, start, turns, thick, mat, y=-0.05, flip=1):
    """Filigree scroll: a spiral tube that thins towards its centre."""
    cu, o = _tube("curl", thick, mat)
    sp = cu.splines.new('POLY')
    n = 56
    sp.points.add(n - 1)
    for i in range(n):
        t = i / (n - 1)
        a = start + flip * t * turns * math.tau
        rr = u(r) * (1 - 0.75 * t)
        sp.points[i].co = (u(cx) + math.cos(a) * rr, y, u(cz) + math.sin(a) * rr, 1)
        sp.points[i].radius = 1.0 - 0.6 * t
    return o


def vine(points, thick, mat, y=-0.05):
    """A smooth tapering tube through (x, z) px points, for flourishes."""
    cu, o = _tube("vine", thick, mat)
    sp = cu.splines.new('BEZIER')
    sp.bezier_points.add(len(points) - 1)
    for i, (x, z) in enumerate(points):
        bp = sp.bezier_points[i]
        bp.co = (u(x), y, u(z))
        bp.handle_left_type = bp.handle_right_type = 'AUTO'
        bp.radius = 1.0 - 0.5 * i / max(1, len(points) - 1)
    return o


def engraved_band_h(x0, x1, z, m, y=-0.02):
    """Repeating engraving along a horizontal band: a groove, a bronze
    diamond and a gold stud every PERIOD px, aligned to x = 0 so the middle
    segment of a 9-slice tiles seamlessly."""
    box(x0, z - 1.5, x1, z + 1.5, 0.05, m["iron_dark"], y=y + 0.03)
    for k in range(math.ceil((x0 - PERIOD / 2) / PERIOD), math.floor((x1 - PERIOD / 2) / PERIOD) + 1):
        x = PERIOD / 2 + k * PERIOD
        diamond(x, z, 4.2, 0.06, m["bronze"], y=y - 0.01, bevel=0.006)
        stud(x - PERIOD / 2, z, 1.6, m["gold"], y=y - 0.02)


def engraved_band_v(z0, z1, x, m, y=-0.02):
    box(x - 1.5, z0, x + 1.5, z1, 0.05, m["iron_dark"], y=y + 0.03)
    for k in range(math.ceil((z0 - PERIOD / 2) / PERIOD), math.floor((z1 - PERIOD / 2) / PERIOD) + 1):
        z = PERIOD / 2 + k * PERIOD
        diamond(x, z, 4.2, 0.06, m["bronze"], y=y - 0.01, bevel=0.006)
        stud(x, z - PERIOD / 2, 1.6, m["gold"], y=y - 0.02)


def corner_bracket(cx, cz, sx, sz, m, arm=104, width=26, gem="ruby"):
    """Heavy bronze L-bracket with a jewel boss, a spike and filigree.
    (cx, cz) is the outer corner, (sx, sz) point outwards."""
    ix, iz = -sx, -sz
    box(cx, cz, cx + ix * arm, cz + iz * width, 0.14, m["bronze"], y=-0.06, bevel=0.02)
    box(cx, cz, cx + ix * width, cz + iz * arm, 0.14, m["bronze"], y=-0.06, bevel=0.02)
    box(cx + ix * 6, cz + iz * (width / 2 - 1), cx + ix * (arm - 10), cz + iz * (width / 2 + 1), 0.05, m["iron_dark"], y=-0.13)
    box(cx + ix * (width / 2 - 1), cz + iz * 6, cx + ix * (width / 2 + 1), cz + iz * (arm - 10), 0.05, m["iron_dark"], y=-0.13)
    diamond(cx + ix * arm, cz + iz * width / 2, width * 0.45, 0.12, m["bronze"], y=-0.07)
    diamond(cx + ix * width / 2, cz + iz * arm, width * 0.45, 0.12, m["bronze"], y=-0.07)
    curl(cx + ix * (arm - 22), cz + iz * (width + 10), 10, math.pi / 2 * (1 if iz > 0 else -1), 1.3, 3.0, m["gold"], y=-0.1, flip=sx * sz)
    curl(cx + ix * (width + 10), cz + iz * (arm - 22), 10, 0 if ix > 0 else math.pi, 1.3, 3.0, m["gold"], y=-0.1, flip=-sx * sz)
    bx, bz = cx + ix * width * 0.9, cz + iz * width * 0.9
    diamond(bx, bz, 30, 0.18, m["bronze"], y=-0.1, bevel=0.025)
    diamond(bx, bz, 21, 0.2, m["iron"], y=-0.12, bevel=0.015)
    jewel(bx, bz, 11, m[gem], m["gold"], y=-0.2)
    spike(cx + ix * 4, cz + iz * 4, math.atan2(sz, sx), 20, 7, m["gold"], y=-0.1)
    for d in (1, 2, 3):
        stud(cx + ix * (width + 18 + d * 14), cz + iz * (width + 6), 2.6, m["gold"], y=-0.1)
        stud(cx + ix * (width + 6), cz + iz * (width + 18 + d * 14), 2.6, m["gold"], y=-0.1)

# ---------------------------------------------------------------- parts
# Each part returns (w, h, margins [l, t, r, b] or None, anchors). Anchor
# rects are (x0, z0, x1, z1) and circles (x, z, r) in px from the centre;
# they are converted to image px (from the top-left) for the manifest.

def part_window(m):
    W = H = 384
    hw = hh = W / 2
    box(-hw + 20, -hh + 20, hw - 20, hh - 20, 0.04, m["stone"], y=0.12)
    frame(-hw + 4, -hh + 4, hw - 4, hh - 4, 48, 0.16, m["iron"], bevel=0.025)
    frame(-hw + 8, -hh + 8, hw - 8, hh - 8, 5, 0.18, m["bronze"], y=-0.02, bevel=0.006)
    frame(-hw + 44, -hh + 44, hw - 44, hh - 44, 6, 0.17, m["bronze"], y=-0.02, bevel=0.008)
    frame(-hw + 50, -hh + 50, hw - 50, hh - 50, 4, 0.08, m["iron_dark"], y=0.06, bevel=0.003)
    c = hw - 29  # centre line of the engraved band
    for s in (-1, 1):
        engraved_band_h(-hw + 60, hw - 60, s * c, m)
        engraved_band_v(-hh + 60, hh - 60, s * c, m)
    for sx in (-1, 1):
        for sz in (-1, 1):
            corner_bracket(sx * (hw - 2), sz * (hh - 2), sx, sz, m)
    return W, H, [128, 128, 128, 128], {"tile": True, "content": 56}


def part_crest(m):
    """Fan crest that sits on top of a window's title plaque."""
    W, H = 320, 160
    base_z = -62
    fan(0, base_z, 84, m["iron"], y=0.06)
    fan(0, base_z, 88, m["bronze"], y=0.1)
    for i in range(11):  # radiating ridges of the shell
        a = math.radians(20 + i * 14)
        L = 110 if i % 2 == 0 else 92
        x1, z1 = math.cos(a) * L, base_z + math.sin(a) * L
        vine([(0, base_z), (x1 * 0.55, base_z + (z1 - base_z) * 0.6), (x1, z1)], 6 if i % 2 == 0 else 4.5,
             m["bronze"] if i % 2 == 0 else m["gold"], y=-0.02)
    disc(0, base_z, 34, 0.14, m["bronze"], y=-0.02, bevel=0.02)
    jewel(0, base_z + 6, 16, m["ruby"], m["gold"], y=-0.12)
    for s in (-1, 1):
        curl(s * 76, base_z + 10, 22, math.pi / 2, 1.4, 4.2, m["bronze"], y=-0.04, flip=s)
        curl(s * 124, base_z + 2, 14, math.pi, 1.3, 3.2, m["gold"], y=-0.05, flip=-s)
    box(-146, base_z - 16, 146, base_z - 4, 0.14, m["iron"], y=0.0, bevel=0.02)
    return W, H, None, {}


def part_plaque(m):
    W, H = 400, 96
    hw, hh = W / 2, H / 2
    box(-hw + 64, -hh + 16, hw - 64, hh - 16, 0.1, m["iron"], bevel=0.03)
    frame(-hw + 64, -hh + 16, hw - 64, hh - 16, 5, 0.12, m["bronze"], y=-0.01, bevel=0.006)
    box(-hw + 80, -hh + 26, hw - 80, hh - 26, 0.06, m["stone_dark"], y=-0.035)
    for sx in (-1, 1):
        x = sx * (hw - 44)
        diamond(x, 0, 36, 0.16, m["bronze"], y=-0.03, bevel=0.02)
        diamond(x, 0, 26, 0.18, m["iron"], y=-0.05, bevel=0.012)
        jewel(x, 0, 11, m["ruby"], m["gold"], y=-0.14)
        spike(x + sx * 34, 0, 0 if sx > 0 else math.pi, 18, 6, m["gold"])
    return W, H, [120, 30, 120, 30], {"content": 12}


def part_panel(m):
    W = H = 128
    hw = hh = W / 2
    box(-hw + 6, -hh + 6, hw - 6, hh - 6, 0.04, m["stone_dark"], y=0.08)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 9, 0.08, m["iron"], bevel=0.01)
    frame(-hw + 10, -hh + 10, hw - 10, hh - 10, 2, 0.05, m["bronze"], y=0.01, bevel=0.002)
    for sx in (-1, 1):
        for sz in (-1, 1):
            stud(sx * (hw - 5), sz * (hh - 5), 3.2, m["gold"], y=-0.05)
    return W, H, [24, 24, 24, 24], {"content": 8}


def part_card(m):
    W = H = 160
    hw = hh = W / 2
    box(-hw + 6, -hh + 6, hw - 6, hh - 6, 0.06, m["stone_mid"], y=0.06, bevel=0.01)
    frame(-hw + 2, -hh + 2, hw - 2, hh - 2, 7, 0.1, m["iron"], bevel=0.01)
    frame(-hw + 9, -hh + 9, hw - 9, hh - 9, 2, 0.06, m["bronze"], y=0.0, bevel=0.002)
    for sx in (-1, 1):
        for sz in (-1, 1):
            cx, cz = sx * (hw - 3), sz * (hh - 3)
            box(cx, cz, cx - sx * 30, cz - sz * 9, 0.12, m["bronze"], y=-0.05, bevel=0.01)
            box(cx, cz, cx - sx * 9, cz - sz * 30, 0.12, m["bronze"], y=-0.05, bevel=0.01)
            stud(cx - sx * 5, cz - sz * 5, 3.4, m["gold"], y=-0.12)
    return W, H, [40, 40, 40, 40], {"content": 10}


BUTTONS = {
    "gray": (0.11, 0.105, 0.1), "orange": (0.8, 0.36, 0.06), "red": (0.5, 0.05, 0.03),
    "green": (0.07, 0.3, 0.1), "blue": (0.06, 0.18, 0.5), "purple": (0.28, 0.07, 0.42),
}


def part_button(m, kind, state):
    W, H = 200, 76
    hw, hh = W / 2, H / 2
    base = BUTTONS[kind]
    off = state == "disabled"
    if off:
        g = sum(base) / 3 * 0.5
        base = (g, g, g)
    # an enamelled, pillow-shaped face in a bronze bezel
    face = C.weathered(f"face_{kind}_{state}", base, metal=0.35 if off else 0.6, rough=0.3, bump=0.06,
                       dirt=0.55, wear=0.5, hammered=False,
                       emit=[min(1, c * 1.6) for c in base] if state == "hover" else None, strength=0.35)
    y = 0.05 if state == "pressed" else 0.0
    box(-hw + 8, -hh + 8, hw - 8, hh - 8, 0.14, face, y=y, bevel=0.09, segments=5)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 8, 0.2, m["iron"] if off else m["bronze"], y=0.03, bevel=0.015)
    frame(-hw + 8, -hh + 8, hw - 8, hh - 8, 1.5, 0.18, m["iron_dark"], y=0.02)
    frame(-hw + 15, -hh + 15, hw - 15, hh - 15, 1.2, 0.05, m["iron"] if off else m["gold"], y=y - 0.08)
    for sx in (-1, 1):
        diamond(sx * (hw - 7), 0, 9, 0.14, m["iron"] if off else m["bronze"], y=-0.04)
        for sz in (-1, 1):
            stud(sx * (hw - 13), sz * (hh - 12), 3.0, m["iron"] if off else m["gold"], y=-0.08 + y)
    return W, H, [32, 26, 32, 26], {"content": 6}


def part_tab(m, active):
    W, H = 200, 68
    hw, hh = W / 2, H / 2
    box(-hw + 6, -hh, hw - 6, hh - 6, 0.14, m["bronze"] if active else m["iron"], bevel=0.04)
    frame(-hw + 1, -hh - 12, hw - 1, hh - 1, 5, 0.2, m["iron_dark"], y=0.03, bevel=0.01)
    if active:
        box(-hw + 20, hh - 12, hw - 20, hh - 8, 0.16, m["gold"], y=-0.06, bevel=0.004)
    for sx in (-1, 1):
        stud(sx * (hw - 13), hh - 13, 2.8, m["gold"], y=-0.08)
    return W, H, [30, 22, 30, 10], {"content": 6}


def part_side_tab(m, active):
    """Square tab for vertical tab strips (an icon sits in the middle)."""
    W = H = 120
    hw = hh = W / 2
    box(-hw + 6, -hh + 6, hw - 6, hh - 6, 0.14, m["bronze"] if active else m["iron"], bevel=0.04)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 5, 0.2, m["iron_dark"], y=0.03, bevel=0.01)
    box(-hw + 16, -hh + 16, hw - 16, hh - 16, 0.08, m["stone_dark"], y=-0.02, bevel=0.01)
    if active:
        box(hw - 10, -hh + 18, hw - 5, hh - 18, 0.18, m["gold"], y=-0.06, bevel=0.004)
    return W, H, [30, 30, 30, 30], {"content": 8}


RARITY = {
    "empty": (0.07, 0.066, 0.063), "common": (0.3, 0.29, 0.28), "uncommon": (0.14, 0.4, 0.18),
    "rare": (0.14, 0.3, 0.7), "epic": (0.4, 0.16, 0.62), "legendary": (0.85, 0.5, 0.12), "mythic": (0.75, 0.1, 0.08),
}


def part_slot(m, rarity):
    W = H = 128
    hw = hh = W / 2
    rim = C.weathered(f"rim_{rarity}", RARITY[rarity], rough=0.3, bump=0.2, dirt=0.75, wear=0.8)
    box(-hw + 14, -hh + 14, hw - 14, hh - 14, 0.04, m["stone_dark"], y=0.14)
    box(-hw + 10, -hh + 10, hw - 10, hh - 10, 0.1, m["iron_dark"], y=0.1, bevel=0.02)
    frame(-hw + 2, -hh + 2, hw - 2, hh - 2, 11, 0.16, rim, bevel=0.018)
    for sx in (-1, 1):
        for sz in (-1, 1):
            diamond(sx * (hw - 7), sz * (hh - 7), 6, 0.12, m["iron"] if rarity == "empty" else m["gold"], y=-0.07)
    gems = {"epic": "amethyst", "legendary": "amber", "mythic": "ruby"}
    if rarity in gems:
        jewel(0, hh - 7, 5, m[gems[rarity]], m["gold"], y=-0.12)
    return W, H, [26, 26, 26, 26], {"content": 4}


def part_bar_frame(m):
    W, H = 256, 52
    hw, hh = W / 2, H / 2
    box(-hw + 8, -hh + 8, hw - 8, hh - 8, 0.04, m["stone_dark"], y=0.08)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 8, 0.12, m["iron"], bevel=0.012)
    frame(-hw + 9, -hh + 9, hw - 9, hh - 9, 1.5, 0.06, m["bronze"], y=0.0)
    for sx in (-1, 1):
        diamond(sx * (hw - 5), 0, 12, 0.16, m["bronze"], y=-0.05)
        stud(sx * (hw - 5), 0, 3.2, m["gold"], y=-0.14)
    return W, H, [24, 16, 24, 16], {"content": 0}


BARS = {"red": (0.85, 0.06, 0.05), "blue": (0.1, 0.35, 0.95), "green": (0.12, 0.7, 0.22),
        "gold": (0.95, 0.62, 0.1), "purple": (0.55, 0.15, 0.9)}


def part_bar(m, color):
    W, H = 96, 34
    c = BARS[color]
    glass = bpy.data.materials.new(f"glass_{color}")
    b = glass.node_tree.nodes["Principled BSDF"]
    b.inputs["Base Color"].default_value = (*c, 1)
    b.inputs["Roughness"].default_value = 0.08
    b.inputs["Coat Weight"].default_value = 1.0
    b.inputs["Emission Color"].default_value = (*c, 1)
    b.inputs["Emission Strength"].default_value = 0.35
    core = C.material(f"core_{color}", c, emit=tuple(min(1, x * 1.4 + 0.1) for x in c), strength=1.2)
    o = C.add("cylinder", vertices=40, radius=u(H / 2 - 1), depth=u(W + 60), location=(0, 0, 0))
    o.rotation_euler = (0, math.pi / 2, 0)
    o.scale = (1, 0.55, 1)
    C.finish(o, glass)
    k = C.add("cylinder", vertices=24, radius=u(H / 5), depth=u(W + 60), location=(0, 0.02, u(-2)))
    k.rotation_euler = (0, math.pi / 2, 0)
    C.finish(k, core)
    return W, H, [10, 0, 10, 0], {}


def part_status(m):
    """Metin2-style status plate: portrait orb, name channel and two bar
    channels (hp, xp) plus a level badge socket."""
    W, H = 820, 256
    ox, oz, orad = -280, 0, 96
    box(-190, -84, 260, 84, 0.14, m["iron"], y=0.02, bevel=0.03)
    box(-190, 78, 260, 84, 0.16, m["bronze"], y=0.0, bevel=0.008)
    box(-190, -84, 260, -78, 0.16, m["bronze"], y=0.0, bevel=0.008)
    disc(260, 0, 84, 0.14, m["iron"], y=0.035, bevel=0.03)   # just behind the plate: no coplanar faces
    ring(260, 0, 82, 5, m["bronze"], y=-0.02)
    diamond(260, 0, 34, 0.16, m["bronze"], y=-0.03, bevel=0.02)
    jewel(260, 0, 16, m["sapphire"], m["gold"], y=-0.14)
    spike(338, 0, 0, 26, 8, m["gold"])
    anchors = {}
    for name, z0, z1 in (("name", 36, 68), ("hp", -4, 24), ("xp", -44, -20)):
        box(-150, z0, 210, z1, 0.08, m["stone_dark"], y=-0.02, bevel=0.01)
        frame(-152, z0 - 2, 212, z1 + 2, 2, 0.1, m["bronze"], y=-0.03)
        anchors[name] = [-150, z0, 210, z1]
    engraved_band_h(-140, 200, -66, m, y=-0.06)
    # portrait orb with a jewelled bronze ring
    disc(ox, oz, orad + 26, 0.2, m["iron"], y=-0.02, bevel=0.03)
    ring(ox, oz, orad + 14, 11, m["bronze"], y=-0.08)
    ring(ox, oz, orad + 1, 4, m["gold"], y=-0.1)
    disc(ox, oz, orad, 0.1, m["stone_dark"], y=-0.08)
    for i in range(8):
        a = i * math.pi / 4 + math.pi / 8
        diamond(ox + math.cos(a) * (orad + 14), oz + math.sin(a) * (orad + 14), 8, 0.14, m["gold"], y=-0.14)
    for i, g in enumerate(("ruby", "sapphire", "emerald", "amber")):
        a = i * math.pi / 2
        jewel(ox + math.cos(a) * (orad + 14), oz + math.sin(a) * (orad + 14), 8, m[g], m["gold"], y=-0.18)
    bx, bz = ox + 76, oz - 80
    disc(bx, bz, 30, 0.2, m["iron"], y=-0.2, bevel=0.02)
    ring(bx, bz, 28, 5, m["bronze"], y=-0.32)
    disc(bx, bz, 22, 0.1, m["stone_dark"], y=-0.28)
    anchors["orb"] = [ox, oz, orad]
    anchors["badge"] = [bx, bz, 22]
    return W, H, None, anchors


def part_minimap(m):
    W = H = 400
    r = 150
    ring(0, 0, r + 44, 6, m["iron"], y=0.0)
    ring(0, 0, r + 22, 18, m["bronze"], y=-0.02)
    ring(0, 0, r + 3, 5, m["gold"], y=-0.06)
    for i in range(12):
        a = i * math.pi / 6
        if i == 3:
            continue  # north marker
        big = i % 3 == 0
        diamond(math.cos(a) * (r + 22), math.sin(a) * (r + 22), 13 if big else 8, 0.16, m["bronze"] if big else m["gold"], y=-0.1)
        if big:
            jewel(math.cos(a) * (r + 22), math.sin(a) * (r + 22), 8, m["sapphire"], m["gold"], y=-0.18)
    spike(0, r + 12, math.pi / 2, 40, 13, m["gold"], y=-0.14)
    jewel(0, r + 22, 10, m["ruby"], m["gold"], y=-0.22)
    # zone plaque at the bottom
    box(-120, -r - 58, 120, -r - 20, 0.14, m["iron"], y=-0.12, bevel=0.02)
    frame(-120, -r - 58, 120, -r - 20, 4, 0.16, m["bronze"], y=-0.13, bevel=0.005)
    box(-108, -r - 50, 108, -r - 28, 0.06, m["stone_dark"], y=-0.175)
    for s in (-1, 1):
        diamond(s * 124, -r - 39, 17, 0.16, m["bronze"], y=-0.14)
        stud(s * 124, -r - 39, 4, m["gold"], y=-0.22)
    return W, H, None, {"hole": [0, 0, r], "zone": [-108, -r - 50, 108, -r - 28]}


def part_orb(m, big):
    W = H = 320 if big else 176
    r = 124 if big else 70
    ring(0, 0, r + 8, 14 if big else 9, m["bronze"], y=-0.02)
    ring(0, 0, r - 5, 4, m["gold"], y=-0.06)
    n = 8 if big else 4
    for i in range(n):
        a = i * math.tau / n + math.pi / n
        diamond(math.cos(a) * (r + 8), math.sin(a) * (r + 8), 11 if big else 7, 0.16, m["gold"], y=-0.1)
        if big:
            spike(math.cos(a) * (r + 18), math.sin(a) * (r + 18), a, 18, 6, m["bronze"])
    for i in range(4):
        a = i * math.pi / 2
        jewel(math.cos(a) * (r + 8), math.sin(a) * (r + 8), 9 if big else 6,
              m["ruby"] if i % 2 == 0 else m["sapphire"], m["gold"], y=-0.14)
    return W, H, None, {"hole": [0, 0, r - 7]}


def part_icon_frame(m, pressed):
    """Square frame for menu / skill buttons (the icon sits inside)."""
    W = H = 136
    hw = hh = W / 2
    y = 0.04 if pressed else 0.0
    box(-hw + 12, -hh + 12, hw - 12, hh - 12, 0.04, m["stone_dark"], y=0.12)
    frame(-hw + 2, -hh + 2, hw - 2, hh - 2, 12, 0.16, m["iron"], y=y, bevel=0.02)
    frame(-hw + 13, -hh + 13, hw - 13, hh - 13, 2.5, 0.08, m["gold"] if pressed else m["bronze"], y=y - 0.01, bevel=0.002)
    for sx in (-1, 1):
        for sz in (-1, 1):
            cx, cz = sx * (hw - 4), sz * (hh - 4)
            box(cx, cz, cx - sx * 34, cz - sz * 8, 0.14, m["bronze"], y=-0.05 + y, bevel=0.01)
            box(cx, cz, cx - sx * 8, cz - sz * 34, 0.14, m["bronze"], y=-0.05 + y, bevel=0.01)
            stud(cx - sx * 4, cz - sz * 4, 3.4, m["gold"], y=-0.12 + y)
    return W, H, [34, 34, 34, 34], {"content": 10}


def part_chat(m):
    W = H = 128
    hw = hh = W / 2
    glass = bpy.data.materials.new("chatbg")
    b = glass.node_tree.nodes["Principled BSDF"]
    b.inputs["Base Color"].default_value = (0.01, 0.009, 0.008, 1)
    b.inputs["Roughness"].default_value = 0.9
    b.inputs["Alpha"].default_value = 0.7
    box(-hw + 5, -hh + 5, hw - 5, hh - 5, 0.02, glass, y=0.08)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 5, 0.06, m["iron"], bevel=0.006)
    frame(-hw + 6, -hh + 6, hw - 6, hh - 6, 1.5, 0.03, m["bronze"], y=0.0)
    for sx in (-1, 1):
        for sz in (-1, 1):
            diamond(sx * (hw - 4), sz * (hh - 4), 6, 0.1, m["bronze"], y=-0.04)
    return W, H, [20, 20, 20, 20], {"content": 8}


def part_tooltip(m):
    W = H = 160
    hw = hh = W / 2
    box(-hw + 6, -hh + 6, hw - 6, hh - 6, 0.04, m["stone"], y=0.08)
    frame(-hw + 2, -hh + 2, hw - 2, hh - 2, 6, 0.1, m["iron"], bevel=0.008)
    frame(-hw + 8, -hh + 8, hw - 8, hh - 8, 2, 0.05, m["gold"], y=0.0)
    for sx in (-1, 1):
        for sz in (-1, 1):
            curl(sx * (hw - 24), sz * (hh - 14), 8, 0 if sx < 0 else math.pi, 1.2, 2.2, m["gold"], y=-0.06, flip=sx * sz)
            diamond(sx * (hw - 5), sz * (hh - 5), 7, 0.12, m["bronze"], y=-0.05)
    return W, H, [36, 36, 36, 36], {"content": 10}


def part_input(m):
    W, H = 200, 64
    hw, hh = W / 2, H / 2
    box(-hw + 6, -hh + 6, hw - 6, hh - 6, 0.04, m["stone_dark"], y=0.1)
    frame(-hw + 1, -hh + 1, hw - 1, hh - 1, 6, 0.1, m["iron"], bevel=0.01)
    frame(-hw + 7, -hh + 7, hw - 7, hh - 7, 1.5, 0.05, m["bronze"], y=0.02)
    return W, H, [20, 20, 20, 20], {"content": 6}


def part_pill(m):
    W, H = 240, 64
    hw, hh = W / 2, H / 2
    box(-hw + 30, -hh + 8, hw - 12, hh - 8, 0.08, m["stone_dark"], y=0.06, bevel=0.04)
    frame(-hw + 26, -hh + 4, hw - 8, hh - 4, 5, 0.12, m["iron"], bevel=0.02)
    disc(-hw + 30, 0, 29, 0.18, m["iron"], y=-0.02, bevel=0.02)
    ring(-hw + 30, 0, 27, 4, m["bronze"], y=-0.08)
    disc(-hw + 30, 0, 22, 0.1, m["stone_dark"], y=-0.07)
    return W, H, [64, 22, 20, 22], {"content": 4, "socket": [-hw + 30, 0, 22]}


def part_close(m):
    W = H = 112
    disc(0, 0, 44, 0.16, m["iron"], bevel=0.03)
    ring(0, 0, 44, 6, m["bronze"], y=-0.04)
    g = C.add("uv_sphere", radius=u(34), location=(0, -0.02, 0), segments=32, ring_count=16)
    g.scale = (1, 0.45, 1)
    C.finish(g, C.weathered("closegem", (0.45, 0.04, 0.03), metal=0.3, rough=0.2, bump=0.05, dirt=0.4, wear=0.4, hammered=False))
    for a in (math.pi / 4, -math.pi / 4):
        bar = C.add("cube", size=1, location=(0, -0.2, 0))
        bar.scale = (u(46), 0.06, u(9))
        bar.rotation_euler = (0, a, 0)
        C.finish(bar, m["gold"], bevel=0.012)
    for i in range(4):
        a = i * math.pi / 2 + math.pi / 4
        stud(math.cos(a) * 44, math.sin(a) * 44, 4, m["gold"], y=-0.08)
    return W, H, None, {}


def part_separator(m):
    W, H = 512, 40
    box(-236, -3.5, 236, 3.5, 0.06, m["bronze"], bevel=0.008)
    box(-236, -1, 236, 1, 0.07, m["gold"], y=-0.01)
    diamond(0, 0, 14, 0.14, m["bronze"], y=-0.03)
    jewel(0, 0, 7, m["ruby"], m["gold"], y=-0.12)
    for sx in (-1, 1):
        diamond(sx * 244, 0, 9, 0.1, m["bronze"])
        curl(sx * 36, 2, 10, math.pi if sx > 0 else 0, 1.2, 2.4, m["gold"], y=-0.06, flip=sx)
    return W, H, None, {}


PARTS = {
    "window": part_window, "crest": part_crest, "plaque": part_plaque, "panel": part_panel, "card": part_card,
    "bar_frame": part_bar_frame, "status": part_status, "minimap": part_minimap, "chat": part_chat,
    "tooltip": part_tooltip, "input": part_input, "pill": part_pill, "close": part_close, "separator": part_separator,
    "orb_big": lambda m: part_orb(m, True), "orb_small": lambda m: part_orb(m, False),
    "icon_frame": lambda m: part_icon_frame(m, False), "icon_frame_on": lambda m: part_icon_frame(m, True),
    "tab": lambda m: part_tab(m, False), "tab_on": lambda m: part_tab(m, True),
    "side_tab": lambda m: part_side_tab(m, False), "side_tab_on": lambda m: part_side_tab(m, True),
}
for _k in BUTTONS:
    for _s in ("normal", "hover", "pressed", "disabled"):
        PARTS[f"button_{_k}_{_s}"] = (lambda k, s: (lambda m: part_button(m, k, s)))(_k, _s)
for _r in RARITY:
    PARTS[f"slot_{_r}"] = (lambda r: (lambda m: part_slot(m, r)))(_r)
for _c in BARS:
    PARTS[f"bar_{_c}"] = (lambda c: (lambda m: part_bar(m, c)))(_c)


def to_image_px(anchors, w, h):
    out = {}
    for k, v in anchors.items():
        if not isinstance(v, list):
            out[k] = v
        elif len(v) == 4:   # rect -> [x, y, w, h]
            out[k] = [v[0] + w / 2, h / 2 - v[3], v[2] - v[0], v[3] - v[1]]
        elif len(v) == 3:   # circle -> [cx, cy, r]
            out[k] = [v[0] + w / 2, h / 2 - v[1], v[2]]
    return out


def render_part(name, raw):
    # the size is decided by the part itself: build once to learn it
    C.reset(8, 8)
    w, h, margins, anchors = PARTS[name](mats())
    scene(w, h)
    PARTS[name](mats())
    C.render(raw / f"{name}.png")
    return {"size": [w, h], "margin": margins, "anchors": to_image_px(anchors, w, h)}


def pack_atlas(raw, meta, out):
    """Shelf-pack the parts into one atlas (2 px gutter)."""
    items = sorted(meta.items(), key=lambda kv: (-kv[1]["size"][1], kv[0]))
    width = 2048
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
    parts = {n: {"rect": rects[n], "margin": meta[n]["margin"], "anchors": meta[n].get("anchors", {})} for n in meta}
    (out / "gui.json").write_text(json.dumps({"scale": 0.5, "period": PERIOD, "parts": parts}, indent=1))
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
