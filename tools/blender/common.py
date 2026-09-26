"""Shared Blender helpers for the OMMRPG art pipeline (Blender 5.x, bpy).

One lighting rig and one camera convention for everything, so every asset
matches: the key light always comes from the upper left (the direction
the LPC character art is lit from), with a cool fill and a warm rim.
"""
import math
import random

import bpy
from mathutils import Matrix, Vector

KEY_DIR = Vector((-0.55, -0.45, 0.7)).normalized()  # light travels from upper left / front


def reset(res_x, res_y, samples=32, transparent=True):
    bpy.ops.wm.read_factory_settings(use_empty=True)
    sc = bpy.context.scene
    sc.render.engine = 'CYCLES'
    sc.cycles.device = 'CPU'
    sc.cycles.samples = samples
    sc.cycles.use_denoising = True
    sc.cycles.max_bounces = 6
    sc.render.film_transparent = transparent
    sc.render.resolution_x = res_x
    sc.render.resolution_y = res_y
    sc.render.resolution_percentage = 100
    sc.view_settings.view_transform = 'Standard'
    sc.view_settings.exposure = -0.2
    sc.render.image_settings.file_format = 'PNG'
    sc.render.image_settings.color_mode = 'RGBA'
    return sc


def world(sky=(1.0, 0.92, 0.8), ground=(0.18, 0.15, 0.13), strength=0.8):
    """Studio environment: bright warm top, dark floor (metals reflect it)."""
    sc = bpy.context.scene
    w = bpy.data.worlds.new("world")
    sc.world = w
    nt = w.node_tree
    nt.nodes.clear()
    tc = nt.nodes.new("ShaderNodeTexCoord")
    sep = nt.nodes.new("ShaderNodeSeparateXYZ")
    ramp = nt.nodes.new("ShaderNodeValToRGB")
    bg = nt.nodes.new("ShaderNodeBackground")
    out = nt.nodes.new("ShaderNodeOutputWorld")
    nt.links.new(tc.outputs["Generated"], sep.inputs[0])
    nt.links.new(sep.outputs["Z"], ramp.inputs[0])
    ramp.color_ramp.elements[0].position = 0.38
    ramp.color_ramp.elements[0].color = (*ground, 1)
    ramp.color_ramp.elements[1].position = 0.62
    ramp.color_ramp.elements[1].color = (*sky, 1)
    nt.links.new(ramp.outputs[0], bg.inputs[0])
    bg.inputs[1].default_value = strength
    nt.links.new(bg.outputs[0], out.inputs[0])


def _aim(obj, target=Vector((0, 0, 0))):
    obj.rotation_euler = (target - obj.location).to_track_quat('-Z', 'Y').to_euler()


def sun_rig(strength=3.2):
    """Outdoor props: one sun from the upper left plus the sky."""
    bpy.ops.object.light_add(type='SUN', location=(0, 0, 10))
    sun = bpy.context.object
    sun.data.energy = strength
    sun.data.angle = math.radians(8)
    sun.data.color = (1.0, 0.95, 0.85)
    sun.rotation_euler = (-KEY_DIR).to_track_quat('-Z', 'Y').to_euler()
    world(sky=(0.75, 0.82, 1.0), ground=(0.25, 0.22, 0.18), strength=0.55)


def studio_rig():
    """Icons: key (upper left), cool fill (right), strong warm rim (behind)."""
    def area(loc, energy, color, size):
        bpy.ops.object.light_add(type='AREA', location=loc)
        light = bpy.context.object
        light.data.energy = energy
        light.data.color = color
        light.data.size = size
        _aim(light)
    area(tuple(KEY_DIR * 7), 900, (1.0, 0.93, 0.82), 4)
    area((5, -3, 1.5), 260, (0.55, 0.72, 1.0), 4)
    area((0.5, 5, 4), 1500, (1.0, 0.82, 0.58), 2.5)
    world()


def ortho_camera(scale, elevation_deg=35, target=Vector((0, 0, 0)), shift_up=0.0):
    """Orthographic camera looking north (+Y) and down by `elevation_deg`.
    `shift_up` moves the view up (in screen units) so a prop's base can sit
    at the bottom of the frame."""
    sc = bpy.context.scene
    e = math.radians(elevation_deg)
    direction = Vector((0, math.cos(e), -math.sin(e)))  # where the camera looks
    bpy.ops.object.camera_add(location=target - direction * 30)
    cam = bpy.context.object
    sc.camera = cam
    cam.data.type = 'ORTHO'
    cam.data.ortho_scale = scale
    cam.data.clip_end = 100
    _aim(cam, target)
    up = Vector((0, math.sin(e), math.cos(e)))
    cam.location += up * shift_up
    return cam


def icon_camera(scale, elevation_deg=18, yaw_deg=-18):
    sc = bpy.context.scene
    e, y = math.radians(elevation_deg), math.radians(yaw_deg)
    loc = Vector((math.sin(y) * math.cos(e), -math.cos(y) * math.cos(e), math.sin(e))) * 20
    bpy.ops.object.camera_add(location=loc)
    cam = bpy.context.object
    sc.camera = cam
    cam.data.type = 'ORTHO'
    cam.data.ortho_scale = scale
    _aim(cam)
    return cam


def material(name, color, metal=0.0, rough=0.5, emit=None, strength=0.0,
             transmission=0.0, coat=0.0, noise=0.0, noise_scale=6.0, alpha=1.0):
    """Principled material; `noise` > 0 varies the colour a little (foliage,
    stone, wood) so large surfaces are not flat."""
    m = bpy.data.materials.new(name)
    nt = m.node_tree
    b = nt.nodes["Principled BSDF"]
    b.inputs["Metallic"].default_value = metal
    b.inputs["Roughness"].default_value = rough
    b.inputs["Transmission Weight"].default_value = transmission
    b.inputs["Coat Weight"].default_value = coat
    b.inputs["Alpha"].default_value = alpha
    if emit:
        b.inputs["Emission Color"].default_value = (*emit, 1)
        b.inputs["Emission Strength"].default_value = strength
    if noise > 0:
        tex = nt.nodes.new("ShaderNodeTexNoise")
        tex.inputs["Scale"].default_value = noise_scale
        ramp = nt.nodes.new("ShaderNodeValToRGB")
        dark = tuple(max(0.0, c * (1 - noise)) for c in color)
        light = tuple(min(1.0, c * (1 + noise)) for c in color)
        ramp.color_ramp.elements[0].color = (*dark, 1)
        ramp.color_ramp.elements[1].color = (*light, 1)
        nt.links.new(tex.outputs["Fac"], ramp.inputs[0])
        nt.links.new(ramp.outputs[0], b.inputs["Base Color"])
    else:
        b.inputs["Base Color"].default_value = (*color, 1)
    return m


def finish(obj, mat=None, bevel=0.0, segments=2, smooth=True):
    if mat is not None:
        obj.data.materials.clear()
        obj.data.materials.append(mat)
    if bevel > 0:
        m = obj.modifiers.new("bevel", 'BEVEL')
        m.width = bevel
        m.segments = segments
        m.limit_method = 'ANGLE'
    if smooth:
        for p in obj.data.polygons:
            p.use_smooth = True
        obj.modifiers.new("wn", 'WEIGHTED_NORMAL')
    return obj


def displace(obj, strength=0.15, size=0.4, seed=0):
    tex = bpy.data.textures.new(f"clouds{seed}", type='CLOUDS')
    tex.noise_scale = size
    tex.noise_depth = 2
    m = obj.modifiers.new("disp", 'DISPLACE')
    m.texture = tex
    m.strength = strength
    m.texture_coords = 'OBJECT'
    empty = bpy.data.objects.new(f"dispo{seed}", None)
    bpy.context.collection.objects.link(empty)
    r = random.Random(seed)
    empty.location = (r.uniform(-50, 50), r.uniform(-50, 50), r.uniform(-50, 50))
    m.texture_coords_object = empty
    return obj


def add(kind, **kw):
    """Primitive helper: add("uv_sphere", radius=1, location=(...))."""
    getattr(bpy.ops.mesh, f"primitive_{kind}_add")(**kw)
    return bpy.context.object


def meshes():
    return [o for o in bpy.context.scene.objects if o.type == 'MESH']


def transform_all(matrix):
    # matrix_world is only refreshed on a depsgraph update; without this the
    # objects whose transform was just set would lose it
    bpy.context.view_layer.update()
    for o in meshes():
        o.matrix_world = matrix @ o.matrix_world


def render(path):
    bpy.context.scene.render.filepath = str(path)
    bpy.ops.render.render(write_still=True)
