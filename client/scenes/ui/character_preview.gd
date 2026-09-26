class_name CharacterPreview
extends Control
## Animated preview of an LPC character sheet (walking toward the viewer).

var texture: Texture2D
var anim := "walk"
var dir := "down"
var _t := 0.0
var _loading_url := ""

func _init() -> void:
	custom_minimum_size = Vector2(128, 128)
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func show_recipe(recipe: Dictionary) -> void:
	var url := Sprites.character_url(recipe)
	_loading_url = url
	texture = null
	queue_redraw()
	var tex := await Sprites.fetch(url)
	if url == _loading_url:
		texture = tex
		queue_redraw()

func _process(delta: float) -> void:
	_t += delta
	queue_redraw()

func _draw() -> void:
	var s := minf(size.x, size.y)
	if texture == null:
		draw_arc(size / 2, s * 0.2, _t * 4.0, _t * 4.0 + 4.0, 24, UiKit.MUTED, 4.0)
		return
	var a: Array = Sprites.ANIMS[anim]
	var frames: int = a[2]
	var frame := 1 + int(_t * 9.0) % (frames - 1) if anim == "walk" else int(_t * 9.0) % frames
	var row: int = a[0] + (Sprites.DIRS[dir] if a[1] == 4 else 0)
	var src := Rect2(frame * Sprites.SHEET_FRAME, row * Sprites.SHEET_FRAME, Sprites.SHEET_FRAME, Sprites.SHEET_FRAME)
	draw_texture_rect_region(texture, Rect2((size - Vector2(s, s)) / 2, Vector2(s, s)), src)
