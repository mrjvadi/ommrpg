class_name MonsterNode
extends Node2D
## A monster: procedural creature sprite (4-frame idle strip), name/level
## tag and hp bar. It idles around its spawn tile purely for looks; the
## server only cares about the spawn position.

var id := ""
var spawn := {}
var species := {}
var hp := 1
var max_hp := 1
var dead_until := 0
var targeted := false
var home := Vector2.ZERO
var sprite := Sprite2D.new()
var label := Label.new()
var _t := 0.0
var _phase := 0.0
var _flash := 0.0

func setup(state: Dictionary, sp: Dictionary) -> void:
	id = str(state.id)
	spawn = state.spawn
	species = sp
	home = Vector2(float(spawn.x) + 0.5, float(spawn.y) + 0.5)
	position = home * MapView.TILE
	_phase = float(hash(id) % 1000) / 159.0
	sprite.hframes = 4
	sprite.offset = Vector2(0, -12)
	add_child(sprite)
	label.add_theme_font_size_override("font_size", 10)
	label.add_theme_constant_override("outline_size", 4)
	label.add_theme_color_override("font_outline_color", Color.BLACK)
	var rank := str(spawn.get("rank", "normal"))
	var col := Color("ffffff")
	if rank == "elite":
		col = Color("ffb040")
	elif rank == "boss":
		col = Color("ff5050")
	label.add_theme_color_override("font_color", col)
	label.text = "%s  %d" % [str(sp.get("name", "???")), int(spawn.level)]
	label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	label.size = Vector2(140, 14)
	label.position = Vector2(-70, -48 if not sp.get("big", false) else -60)
	add_child(label)
	if rank == "boss":
		scale = Vector2(1.6, 1.6)
	elif rank == "elite":
		scale = Vector2(1.25, 1.25)
	apply_state(state)
	var tex := await Sprites.fetch(Sprites.creature_url(sp))
	if is_instance_valid(self):
		sprite.texture = tex

func apply_state(state: Dictionary) -> void:
	hp = int(state.get("hp", hp))
	max_hp = max(1, int(state.get("max_hp", max_hp)))
	dead_until = int(state.get("dead_until", 0))
	queue_redraw()

func alive() -> bool:
	if dead_until == -1:
		return false
	if dead_until > 0 and Time.get_unix_time_from_system() * 1000.0 < dead_until:
		return false
	if dead_until > 0:
		dead_until = 0
		hp = max_hp
	return true

func hit(new_hp: int, new_max: int) -> void:
	hp = new_hp
	max_hp = new_max
	_flash = 0.15
	queue_redraw()

## A short elemental burst when hit by an element.
func burst(col: Color) -> void:
	var p := CPUParticles2D.new()
	p.one_shot = true
	p.amount = 14
	p.lifetime = 0.45
	p.explosiveness = 0.9
	p.spread = 180.0
	p.initial_velocity_min = 25.0
	p.initial_velocity_max = 60.0
	p.gravity = Vector2.ZERO
	p.scale_amount_max = 2.0
	p.color = col
	p.position = Vector2(0, -12)
	add_child(p)
	p.emitting = true
	get_tree().create_timer(1.0).timeout.connect(p.queue_free)

func die(until: int) -> void:
	dead_until = until
	hp = 0
	queue_redraw()

func tile_pos() -> Vector2:
	return home

func _process(delta: float) -> void:
	_t += delta
	if not alive():
		visible = false
		return
	visible = true
	sprite.frame = int(_t * 4.0 + _phase) % 4
	var fly: bool = species.get("family", "") in ["bird", "spirit", "elemental", "imp"]
	var wander := Vector2(sin(_t * 0.37 + _phase) * 0.35, cos(_t * 0.29 + _phase * 1.3) * 0.25)
	position = (home + wander) * MapView.TILE
	if fly:
		sprite.offset.y = -12 + sin(_t * 3.0) * 3.0
	sprite.flip_h = cos(_t * 0.37 + _phase) < 0
	if _flash > 0:
		_flash -= delta
		sprite.modulate = Color(3, 3, 3) if _flash > 0 else Color.WHITE

func _draw() -> void:
	if hp < max_hp and hp > 0:
		var w := 30.0
		draw_rect(Rect2(-w / 2, -34, w, 4), Color(0, 0, 0, 0.7))
		draw_rect(Rect2(-w / 2, -34, w * float(hp) / float(max_hp), 4), Color("e05060"))
	if targeted:
		draw_arc(Vector2(0, 2), 14, 0, TAU, 24, Color(1, 0.85, 0.3, 0.9), 2.0)
