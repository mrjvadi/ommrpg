class_name Portrait
extends Control
## Round avatar: the head of the character's LPC sheet (idle, facing the
## viewer) in a bronze ring.

var texture: Texture2D
## Only the picture (a frame drawn by someone else surrounds it).
var bare := false
var _url := ""

func _init(diameter := 84.0) -> void:
	custom_minimum_size = Vector2(diameter, diameter)
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func show_recipe(recipe: Dictionary) -> void:
	var url := Sprites.character_url(recipe)
	if url == _url:
		return
	_url = url
	var tex := await Sprites.fetch(url)
	if url == _url and is_instance_valid(self):
		texture = tex
		queue_redraw()

func _draw() -> void:
	var c := size / 2
	var r := minf(size.x, size.y) / 2
	if bare:
		_draw_bare(c, r)
		return
	draw_circle(c + Vector2(0, 3), r, Color(0, 0, 0, 0.5))
	draw_circle(c, r, UiKit.OUTLINE)
	draw_circle(c, r - 2, Color("c9a064"))
	draw_circle(c, r - 5, Color("6a4424"))
	draw_circle(c, r - 7, Color("3b4a5c"))
	draw_circle(c + Vector2(0, r * 0.25), r - 12, Color("516377"))
	if texture:
		# walk-down row, standing frame; the head sits in the upper half
		var row: int = Sprites.ANIMS.walk[0] + Sprites.DIRS.down
		var f := Sprites.SHEET_FRAME
		var src := Rect2(16, row * f + 6, 32, 30)
		var dst_size := Vector2(r * 1.7, r * 1.6)
		draw_texture_rect_region(texture, Rect2(c - Vector2(dst_size.x / 2, dst_size.y * 0.52), dst_size), src)
	var ring := UiKit.part("orb_small")
	if ring:
		var fr := r * 1.12
		draw_texture_rect(ring, Rect2(c - Vector2(fr, fr), Vector2(fr, fr) * 2), false)
		return
	draw_arc(c, r - 6, 0, TAU, 64, UiKit.OUTLINE, 3.0, true)
	draw_arc(c, r - 3, PI * 1.1, PI * 1.9, 32, Color(1, 1, 1, 0.4), 1.5, true)

func _draw_bare(c: Vector2, r: float) -> void:
	# a dusk-blue backdrop with a soft light behind the head, clipped round
	for i in 8:
		var t := i / 7.0
		draw_circle(c + Vector2(0, r * 0.18 * t), r * (1.0 - 0.55 * t), Color("1c2430").lerp(Color("5b6f86"), t))
	if texture:
		var row: int = Sprites.ANIMS.walk[0] + Sprites.DIRS.down
		var f := Sprites.SHEET_FRAME
		var src := Rect2(16, row * f + 4, 32, 36)
		var img_rect := Rect2(c - Vector2(r * 0.95, r * 0.9), Vector2(r * 1.9, r * 2.14))
		# clip: only rows inside the circle
		var steps := 24
		for i in steps:
			var y0 := img_rect.position.y + img_rect.size.y * i / steps
			var y1 := img_rect.position.y + img_rect.size.y * (i + 1) / steps
			var ym := clampf((y0 + y1) / 2 - c.y, -r, r)
			var half := sqrt(maxf(0.0, r * r - ym * ym))
			var x0 := maxf(img_rect.position.x, c.x - half)
			var x1 := minf(img_rect.end.x, c.x + half)
			if x1 <= x0:
				continue
			var u0 := (x0 - img_rect.position.x) / img_rect.size.x
			var u1 := (x1 - img_rect.position.x) / img_rect.size.x
			var v0 := float(i) / steps
			var v1 := float(i + 1) / steps
			draw_texture_rect_region(texture, Rect2(x0, y0, x1 - x0, y1 - y0),
				Rect2(src.position.x + src.size.x * u0, src.position.y + src.size.y * v0, src.size.x * (u1 - u0), src.size.y * (v1 - v0)))
	draw_arc(c, r - 1, 0, TAU, 64, Color(0, 0, 0, 0.6), 3.0, true)
