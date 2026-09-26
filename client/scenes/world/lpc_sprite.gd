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
var fx_sprite: Sprite2D
var fx_particles: CPUParticles2D
var _fx_key := ""

const FX_SHADER := preload("res://assets/shaders/weapon_fx.gdshader")
const ELEMENT_COLORS := {
	"fire": Color(1.0, 0.47, 0.12), "frost": Color(0.51, 0.84, 1.0), "lightning": Color(0.78, 0.82, 1.0),
	"poison": Color(0.47, 0.94, 0.31), "holy": Color(1.0, 0.94, 0.59), "shadow": Color(0.63, 0.31, 1.0),
}
const ELEMENT_STYLE := {"fire": 1, "frost": 2, "lightning": 3, "poison": 4, "holy": 5, "shadow": 6}

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

## Weapon effect from the server (`fx` = {element, tier, rarity} or null):
## a glow shader over the weapon mask and, from tier 2, element particles.
func set_fx(recipe: Dictionary, fx) -> void:
	var key := "" if not fx is Dictionary else "%s:%s:%s" % [str(fx.get("element", "")), str(fx.get("tier", 0)), Sprites.character_url(recipe)]
	if key == _fx_key:
		return
	_fx_key = key
	if fx_sprite:
		fx_sprite.queue_free()
		fx_sprite = null
	if fx_particles:
		fx_particles.queue_free()
		fx_particles = null
	if key == "":
		return
	var element := str(fx.get("element", ""))
	var tier := int(fx.get("tier", 1))
	var col: Color = ELEMENT_COLORS.get(element, Sprites.rarity_color(str(fx.get("rarity", "rare"))))
	fx_sprite = Sprite2D.new()
	fx_sprite.centered = false
	fx_sprite.offset = sprite.offset
	fx_sprite.region_enabled = true
	var mat := ShaderMaterial.new()
	mat.shader = FX_SHADER
	mat.set_shader_parameter("glow_color", col)
	mat.set_shader_parameter("intensity", [0.0, 0.8, 1.3, 1.9][clampi(tier, 0, 3)])
	mat.set_shader_parameter("radius", [1.0, 1.5, 2.2, 3.0][clampi(tier, 0, 3)])
	mat.set_shader_parameter("style", ELEMENT_STYLE.get(element, 0))
	fx_sprite.material = mat
	add_child(fx_sprite)
	if tier >= 2:
		fx_particles = CPUParticles2D.new()
		fx_particles.amount = 10 if tier == 2 else 22
		fx_particles.lifetime = 0.8
		fx_particles.position = Vector2(0, -30)
		fx_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_RECTANGLE
		fx_particles.emission_rect_extents = Vector2(14, 16)
		fx_particles.direction = Vector2(0, -1)
		fx_particles.spread = 35.0
		fx_particles.gravity = Vector2(0, -25 if element != "frost" else 20)
		fx_particles.initial_velocity_min = 6.0
		fx_particles.initial_velocity_max = 18.0
		fx_particles.scale_amount_min = 1.0
		fx_particles.scale_amount_max = 2.0
		fx_particles.color = col
		var ramp := Gradient.new()
		ramp.set_color(0, col)
		ramp.set_color(1, Color(col, 0.0))
		fx_particles.color_ramp = ramp
		add_child(fx_particles)
	var tex := await Sprites.fetch(Sprites.character_url(recipe) + "&mask=weapon")
	if fx_sprite and _fx_key == key:
		fx_sprite.texture = tex

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
	if fx_sprite:
		fx_sprite.region_rect = sprite.region_rect
