class_name LpcSprite
extends Node2D
## A character drawn from an LPC universal sheet (64x64 frames) with walk,
## idle and one-shot action animations, a name tag and optional hp bar.
## The node's position is the character's feet.

var sprite := Sprite2D.new()
var name_label := Label.new()
var anim := "walk"
var dir := "down"
var moving := false
var _t := 0.0
var _oneshot := ""
var _oneshot_t := 0.0
var _url := ""

func _init() -> void:
	sprite.centered = false
	sprite.offset = Vector2(-32, -60)
	sprite.region_enabled = true
	sprite.region_rect = Rect2(0, 0, 64, 64)
	add_child(sprite)
	name_label.add_theme_font_size_override("font_size", 11)
	name_label.add_theme_color_override("font_outline_color", Color.BLACK)
	name_label.add_theme_constant_override("outline_size", 4)
	name_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	name_label.position = Vector2(-60, -76)
	name_label.size = Vector2(120, 16)
	add_child(name_label)

func set_label(text: String, color := Color.WHITE) -> void:
	name_label.text = text
	name_label.add_theme_color_override("font_color", color)

func set_recipe(recipe: Dictionary) -> void:
	var url := Sprites.character_url(recipe)
	if url == _url:
		return
	_url = url
	var tex := await Sprites.fetch(url)
	if url == _url:
		sprite.texture = tex

## Plays an action animation once (slash, thrust, shoot, spellcast, hurt).
func play_once(a: String) -> void:
	if not Sprites.ANIMS.has(a):
		return
	_oneshot = a
	_oneshot_t = 0.0

func face_towards(v: Vector2) -> void:
	if v.length() < 0.01:
		return
	if absf(v.x) > absf(v.y):
		dir = "right" if v.x > 0 else "left"
	else:
		dir = "down" if v.y > 0 else "up"

func _process(delta: float) -> void:
	_t += delta
	var a := "walk"
	var frame := 0
	if _oneshot != "":
		_oneshot_t += delta
		var frames: int = Sprites.ANIMS[_oneshot][2]
		frame = int(_oneshot_t * 16.0)
		if frame >= frames:
			_oneshot = ""
			frame = 0
		else:
			a = _oneshot
	if _oneshot == "":
		a = "walk"
		frame = 1 + int(_t * 10.0) % 8 if moving else 0
	var info: Array = Sprites.ANIMS[a]
	var row: int = info[0] + (Sprites.DIRS[dir] if info[1] == 4 else 0)
	sprite.region_rect = Rect2(frame * 64, row * 64, 64, 64)
