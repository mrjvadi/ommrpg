class_name KitPart
extends Control
## Draws one whole part of the GUI kit (status plate, minimap ring, orb
## frames...) at a fixed scale, and maps the part's anchors (see
## Pack.anchors) into this control's coordinates so widgets can be placed
## exactly on the painted channels and sockets.

var part := ""
var scale_k := 0.5
var modulate_part := Color.WHITE
## Used when the kit is unavailable: size and anchors in texture px.
var fallback_size := Vector2.ZERO
var fallback_anchors := {}

func _init(part_name := "", k := 0.5, fb_size := Vector2.ZERO, fb_anchors := {}) -> void:
	part = part_name
	scale_k = k
	fallback_size = fb_size
	fallback_anchors = fb_anchors
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = tex_size() * k

func available() -> bool:
	return Pack.has_part(part)

func tex_size() -> Vector2:
	return Pack.part_size(part) if available() else fallback_size

func _anchor(name: String) -> Array:
	var a = Pack.anchors(part).get(name) if available() else fallback_anchors.get(name)
	return a if a is Array else []

## A rect anchor in local coordinates.
func rect(name: String) -> Rect2:
	var a := _anchor(name)
	if a.size() < 4:
		return Rect2()
	return Rect2(Vector2(a[0], a[1]) * scale_k, Vector2(a[2], a[3]) * scale_k)

## A circle anchor in local coordinates: Vector3(cx, cy, r).
func circle(name: String) -> Vector3:
	var a := _anchor(name)
	if a.size() < 3:
		return Vector3()
	return Vector3(a[0], a[1], a[2]) * scale_k

func _draw() -> void:
	var tex := Pack.part_texture(part)
	if tex:
		draw_texture_rect(tex, Rect2(Vector2.ZERO, size), false, modulate_part)


## Draws a texture clipped to a circle (minimap, portraits).
static func draw_circle_texture(ci: CanvasItem, tex: Texture2D, center: Vector2, r: float, src := Rect2(0, 0, 1, 1), steps := 48) -> void:
	var pts := PackedVector2Array()
	var uvs := PackedVector2Array()
	for i in steps:
		var a := TAU * i / steps
		var d := Vector2(cos(a), sin(a))
		pts.append(center + d * r)
		uvs.append(src.position + (d * 0.5 + Vector2(0.5, 0.5)) * src.size)
	ci.draw_polygon(pts, PackedColorArray([Color.WHITE]), uvs, tex)
