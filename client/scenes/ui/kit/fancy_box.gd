class_name FancyBox
extends StyleBox
## A layered frame drawn in code: drop shadow, dark outline, a metallic rim
## with a vertical gradient and a light top edge, and a gradient fill with
## an optional gloss or inner shadow. Drawn with vector polygons at the
## screen's real resolution, so it stays sharp on every phone (unlike
## stretched bitmaps).
##
##   var b := FancyBox.new()
##   b.rim_top = Color("7a6450"); b.fill_top = Color("2c2826")

var radius := 10.0
var outline := Color("0d0908")
var outline_w := 2.0
var rim_top := Color("6f5b49")
var rim_bottom := Color("3a2e26")
var rim_w := 3.0
var fill_top := Color("2f2a27")
var fill_bottom := Color("1c1918")
var inner_line := Color(0, 0, 0, 0.6)
var highlight := Color(1, 0.93, 0.8, 0.28)
var gloss := 0.0
var inset_shadow := 0.0
var shadow := Color(0, 0, 0, 0.45)
var shadow_offset := Vector2(0, 3)
## Only round these corners (top-left, top-right, bottom-right, bottom-left).
var corners := [true, true, true, true]

func _init(pad := 10.0) -> void:
	set_content_margin_all(pad)

func copy() -> FancyBox:
	var b := FancyBox.new()
	for p in ["radius", "outline", "outline_w", "rim_top", "rim_bottom", "rim_w", "fill_top", "fill_bottom",
			"inner_line", "highlight", "gloss", "inset_shadow", "shadow", "shadow_offset"]:
		b.set(p, get(p))
	b.corners = corners.duplicate()
	for side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]:
		b.set_content_margin(side, get_content_margin(side))
	return b

func _draw(ci: RID, rect: Rect2) -> void:
	var rs := RenderingServer
	var r := minf(radius, minf(rect.size.x, rect.size.y) / 2.0)
	if shadow.a > 0.0:
		var sp := FancyBox.rounded(Rect2(rect.position + shadow_offset, rect.size), r, corners)
		rs.canvas_item_add_polygon(ci, sp, PackedColorArray([shadow]))
	var outer := FancyBox.rounded(rect, r, corners)
	rs.canvas_item_add_polygon(ci, outer, PackedColorArray([outline]))
	var rim_rect := rect.grow(-outline_w)
	var rim_r := maxf(0.0, r - outline_w)
	var fill_rect := rim_rect.grow(-rim_w)
	var fill_r := maxf(0.0, rim_r - rim_w)
	if rim_w > 0.0 and rim_rect.size.x > 0 and rim_rect.size.y > 0:
		var rim := FancyBox.rounded(rim_rect, rim_r, corners)
		rs.canvas_item_add_polygon(ci, rim, FancyBox.gradient(rim, rim_rect, rim_top, rim_bottom))
		if highlight.a > 0.0:
			rs.canvas_item_add_polyline(ci, FancyBox.top_edge(rim, rim_rect), PackedColorArray([highlight]), 1.4, true)
	if fill_rect.size.x > 1 and fill_rect.size.y > 1:
		var fill := FancyBox.rounded(fill_rect, fill_r, corners)
		rs.canvas_item_add_polygon(ci, fill, FancyBox.gradient(fill, fill_rect, fill_top, fill_bottom))
		if inset_shadow > 0.0:
			var band := Rect2(fill_rect.position, Vector2(fill_rect.size.x, minf(fill_rect.size.y, 14.0)))
			var pts := FancyBox.rounded(band, fill_r, [corners[0], corners[1], false, false])
			rs.canvas_item_add_polygon(ci, pts, FancyBox.gradient(pts, band, Color(0, 0, 0, inset_shadow), Color(0, 0, 0, 0)))
		if gloss > 0.0:
			var g := Rect2(fill_rect.position, Vector2(fill_rect.size.x, fill_rect.size.y * 0.48))
			var pts := FancyBox.rounded(g, fill_r, [corners[0], corners[1], false, false])
			rs.canvas_item_add_polygon(ci, pts, FancyBox.gradient(pts, g, Color(1, 1, 1, gloss), Color(1, 1, 1, gloss * 0.15)))
		if inner_line.a > 0.0:
			var loop := fill.duplicate()
			loop.append(fill[0])
			rs.canvas_item_add_polyline(ci, loop, PackedColorArray([inner_line]), 1.2, true)
	var edge := outer.duplicate()
	edge.append(outer[0])
	rs.canvas_item_add_polyline(ci, edge, PackedColorArray([outline]), 1.3, true)

## Rounded rectangle outline as a convex polygon (clockwise).
static func rounded(rect: Rect2, r: float, round_corners := [true, true, true, true]) -> PackedVector2Array:
	var pts := PackedVector2Array()
	r = minf(r, minf(rect.size.x, rect.size.y) / 2.0)
	var seg := clampi(int(r / 1.5), 2, 12)
	# corner centres: top-left, top-right, bottom-right, bottom-left
	var cs := [rect.position + Vector2(r, r), Vector2(rect.end.x - r, rect.position.y + r), rect.end - Vector2(r, r), Vector2(rect.position.x + r, rect.end.y - r)]
	var sharp := [rect.position, Vector2(rect.end.x, rect.position.y), rect.end, Vector2(rect.position.x, rect.end.y)]
	var start_angles := [PI, PI * 1.5, 0.0, PI * 0.5]
	for i in 4:
		if r <= 0.5 or not round_corners[i]:
			pts.append(sharp[i])
			continue
		for s in seg + 1:
			var a: float = start_angles[i] + PI * 0.5 * float(s) / seg
			pts.append(cs[i] + Vector2(cos(a), sin(a)) * r)
	return pts

static func gradient(pts: PackedVector2Array, rect: Rect2, top: Color, bottom: Color) -> PackedColorArray:
	var cols := PackedColorArray()
	cols.resize(pts.size())
	var h := maxf(1.0, rect.size.y)
	for i in pts.size():
		cols[i] = top.lerp(bottom, clampf((pts[i].y - rect.position.y) / h, 0.0, 1.0))
	return cols

## The upper part of a rounded outline (for the light edge on the rim).
static func top_edge(pts: PackedVector2Array, rect: Rect2) -> PackedVector2Array:
	var out := PackedVector2Array()
	var limit := rect.position.y + minf(rect.size.y * 0.35, 12.0)
	# walk from the left side over the top to the right side
	var n := pts.size()
	var first := -1
	for i in n:
		if pts[i].y <= limit and pts[i].x < rect.get_center().x:
			first = i
			break
	if first == -1:
		return out
	var i := first
	for _k in n:
		if pts[i].y > limit:
			break
		out.append(pts[i])
		i = (i + 1) % n
	return out
