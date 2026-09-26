class_name Stage
extends Control
## Decorative backdrop for showing a hero: a stone archway with a night sky,
## a flagstone floor and two flickering torches.

var _t := 0.0

func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	clip_contents = true

func _process(delta: float) -> void:
	_t += delta
	queue_redraw()

func _draw() -> void:
	var w := size.x
	var h := size.y
	# wall
	draw_texture_rect(UiKit.STONE, Rect2(Vector2.ZERO, size), true, Color(0.62, 0.58, 0.56))
	# archway opening with a moonlit sky
	var aw := w * 0.46
	var ax := (w - aw) / 2
	var top := h * 0.1
	var spring := h * 0.36
	var arch := PackedVector2Array()
	var cols := PackedColorArray()
	arch.append(Vector2(ax, h * 0.78))
	cols.append(Color("1e2a3c"))
	for i in 25:
		var a := PI + PI * i / 24.0
		var p := Vector2(w / 2 + cos(a) * aw / 2, spring + sin(a) * (spring - top))
		arch.append(p)
		cols.append(Color("3c5374").lerp(Color("24344c"), (p.y - top) / (h * 0.7)))
	arch.append(Vector2(ax + aw, h * 0.78))
	cols.append(Color("1e2a3c"))
	draw_polygon(arch, cols)
	# distant towers
	for i in 5:
		var tx := ax + aw * (0.12 + i * 0.19)
		var th := h * (0.16 + 0.08 * ((i * 37) % 3))
		draw_rect(Rect2(tx, h * 0.78 - th, aw * 0.1, th), Color("18202d"))
		draw_colored_polygon(PackedVector2Array([Vector2(tx - 3, h * 0.78 - th), Vector2(tx + aw * 0.05, h * 0.78 - th - 14), Vector2(tx + aw * 0.1 + 3, h * 0.78 - th)]), Color("18202d"))
	draw_circle(Vector2(ax + aw * 0.75, top + h * 0.12), h * 0.035, Color(1, 0.97, 0.85, 0.85))
	# arch stones (voussoirs)
	var ring := PackedVector2Array()
	for i in 25:
		var a := PI + PI * i / 24.0
		ring.append(Vector2(w / 2 + cos(a) * (aw / 2 + 8), spring + sin(a) * (spring - top + 8)))
	ring.insert(0, Vector2(ax - 8, h * 0.78))
	ring.append(Vector2(ax + aw + 8, h * 0.78))
	draw_polyline(ring, Color("15100e"), 16.0, true)
	draw_polyline(ring, Color("5b4d43"), 11.0, true)
	for i in range(1, 24, 2):
		var a := PI + PI * i / 24.0
		var p := Vector2(w / 2 + cos(a) * (aw / 2 + 8), spring + sin(a) * (spring - top + 8))
		draw_circle(p, 2.0, Color("15100e"))
	# floor
	var fy := h * 0.78
	draw_rect(Rect2(0, fy, w, h - fy), Color("2a2320"))
	for i in 9:
		var y := fy + (h - fy) * (i / 9.0) * (i / 9.0)
		draw_line(Vector2(0, y), Vector2(w, y), Color(0, 0, 0, 0.35), 1.5)
	for i in range(-8, 9):
		draw_line(Vector2(w / 2 + i * w * 0.05, fy), Vector2(w / 2 + i * w * 0.16, h), Color(0, 0, 0, 0.3), 1.5)
	draw_rect(Rect2(0, fy - 3, w, 5), Color("15100e"))
	# torches
	for side: float in [-1.0, 1.0]:
		var tx: float = w / 2 + side * (aw / 2 + w * 0.08)
		var ty := h * 0.3
		draw_rect(Rect2(tx - 4, ty, 8, 26), Color("2a1a10"))
		draw_rect(Rect2(tx - 9, ty - 4, 18, 8), Color("6a4a2a"))
		var f := sin(_t * 11.0 + side) * 0.5 + sin(_t * 17.0) * 0.3
		for i in 6:
			draw_circle(Vector2(tx, ty - 10), 40.0 - i * 6, Color(1, 0.6, 0.2, 0.035))
		draw_colored_polygon(PackedVector2Array([Vector2(tx - 9, ty - 4), Vector2(tx + f * 3, ty - 30 - f * 4), Vector2(tx + 9, ty - 4)]), Color("ff8a1e"))
		draw_colored_polygon(PackedVector2Array([Vector2(tx - 5, ty - 4), Vector2(tx + f * 2, ty - 20 - f * 3), Vector2(tx + 5, ty - 4)]), Color("ffe27a"))
	# vignette
	var dark := Color(0, 0, 0, 0.6)
	var clear := Color(0, 0, 0, 0)
	draw_polygon(PackedVector2Array([Vector2(0, 0), Vector2(w * 0.2, 0), Vector2(w * 0.2, h), Vector2(0, h)]), PackedColorArray([dark, clear, clear, dark]))
	draw_polygon(PackedVector2Array([Vector2(w * 0.8, 0), Vector2(w, 0), Vector2(w, h), Vector2(w * 0.8, h)]), PackedColorArray([clear, dark, dark, clear]))
