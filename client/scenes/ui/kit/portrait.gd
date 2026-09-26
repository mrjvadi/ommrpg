class_name Portrait
extends Control
## Round avatar: the head of the character's LPC sheet (idle, facing the
## viewer) in a bronze ring.

var texture: Texture2D
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
	draw_arc(c, r - 6, 0, TAU, 64, UiKit.OUTLINE, 3.0, true)
	draw_arc(c, r - 3, PI * 1.1, PI * 1.9, 32, Color(1, 1, 1, 0.4), 1.5, true)
