class_name Ribbon
extends Control
## The red banner with folded tails and golden edges, used behind titles and
## the level / experience bar.

var color := Color("b3342a")
var tails := true

func _init(min_size := Vector2(300, 44)) -> void:
	custom_minimum_size = min_size
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func _notification(what: int) -> void:
	if what == NOTIFICATION_RESIZED:
		queue_redraw()

func _draw() -> void:
	var w := size.x
	var h := size.y
	var t := h * 0.55 if tails else 0.0
	var dark := color.darkened(0.55)
	if tails:
		# folded tails behind the band
		for side: float in [-1.0, 1.0]:
			var x0 := 0.0 if side == -1 else w
			var inner := x0 - side * t * 1.3
			var pts := PackedVector2Array([Vector2(inner, h * 0.2), Vector2(x0, h * 0.2), Vector2(x0 + side * -t * 0.45, h * 0.62), Vector2(x0, h * 1.05), Vector2(inner, h * 1.05)])
			draw_colored_polygon(pts, dark)
			draw_polyline(_closed(pts), UiKit.OUTLINE, 2.0, true)
	var band := PackedVector2Array([Vector2(t * 0.8, 0), Vector2(w - t * 0.8, 0), Vector2(w - t * 0.8, h * 0.86), Vector2(t * 0.8, h * 0.86)])
	var cols := PackedColorArray([color.lightened(0.15), color.lightened(0.15), color.darkened(0.3), color.darkened(0.3)])
	draw_polygon(band, cols)
	# golden edges and pattern dots
	var gold := Color("e8b650")
	draw_line(Vector2(t * 0.8, 3), Vector2(w - t * 0.8, 3), gold, 2.0, true)
	draw_line(Vector2(t * 0.8, h * 0.86 - 3), Vector2(w - t * 0.8, h * 0.86 - 3), gold.darkened(0.3), 2.0, true)
	var x := t * 0.8 + 14
	while x < w - t * 0.8 - 10:
		var cy := h * 0.43
		var d := PackedVector2Array([Vector2(x, cy - 5), Vector2(x + 5, cy), Vector2(x, cy + 5), Vector2(x - 5, cy)])
		draw_colored_polygon(d, Color(1, 0.85, 0.6, 0.12))
		x += 22
	draw_polyline(_closed(band), UiKit.OUTLINE, 2.0, true)

func _closed(p: PackedVector2Array) -> PackedVector2Array:
	var q := p.duplicate()
	q.append(p[0])
	return q
