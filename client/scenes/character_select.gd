extends Control
## Character selection in landscape: the roster on the left, the selected
## (or new) hero on a stage in the middle, and play / create on the right.
## New characters get a seed-generated LPC appearance ("Randomise").

signal play

const MAX_CHARACTERS := 4

var _roster: VBoxContainer
var _side: VBoxContainer
var _status: Label
var _preview: CharacterPreview
var _stage_name: Label
var _name: LineEdit
var _recipe := {}
var _seed := 0
var _body := "male"
var _selected := -1 # index in Game.characters, -1 = creating

func _ready() -> void:
	UiKit.full_rect(self)
	add_child(UiKit.backdrop())
	var m := MarginContainer.new()
	UiKit.full_rect(m)
	add_child(m)
	Telegram.insets_changed.connect(func(): _pad(m))
	get_viewport().size_changed.connect(func(): _pad(m))
	_pad(m)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 14)
	m.add_child(row)

	# roster
	var left := VBoxContainer.new()
	left.custom_minimum_size.x = 260
	left.add_theme_constant_override("separation", 10)
	row.add_child(left)
	left.add_child(UiKit.title("Heroes", 30))
	_roster = VBoxContainer.new()
	_roster.size_flags_vertical = Control.SIZE_EXPAND_FILL
	left.add_child(_roster)
	var tools := HBoxContainer.new()
	tools.add_child(UiKit.button("Settings", func(): Telegram.settings_pressed.emit(), 44, "gray", "settings"))
	var cr := UiKit.button("Credits", func(): Popups.open(preload("res://scenes/ui/credits_panel.gd").new()), 44, "gray", "book")
	tools.add_child(cr)
	left.add_child(tools)

	# stage
	var stage_frame := PanelContainer.new()
	stage_frame.add_theme_stylebox_override("panel", UiKit.inset_box(0))
	stage_frame.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	stage_frame.clip_contents = true
	row.add_child(stage_frame)
	var stage := Control.new()
	stage_frame.add_child(stage)
	stage.add_child(UiKit.full_rect(Stage.new()))
	_preview = CharacterPreview.new()
	stage.add_child(_preview)
	_stage_name = UiKit.label("", 26, UiKit.TEXT, true, 7)
	_stage_name.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	stage.add_child(_stage_name)
	stage.resized.connect(func():
		var s := stage.size
		var pw := minf(s.x * 0.8, s.y * 0.8)
		_preview.size = Vector2(pw, pw)
		_preview.position = Vector2((s.x - pw) / 2, s.y * 0.88 - pw)
		_stage_name.position = Vector2(0, s.y - 48)
		_stage_name.size = Vector2(s.x, 40))

	# right side
	_side = VBoxContainer.new()
	_side.custom_minimum_size.x = 300
	_side.add_theme_constant_override("separation", 10)
	row.add_child(_side)
	_status = UiKit.para("", 16, UiKit.MUTED, true)
	_seed = randi()
	_refresh()

func _pad(m: MarginContainer) -> void:
	var ins := Telegram.insets_for(get_viewport())
	m.add_theme_constant_override("margin_left", int(ins.w + 16))
	m.add_theme_constant_override("margin_right", int(ins.y + 16))
	m.add_theme_constant_override("margin_top", int(ins.x + 12))
	m.add_theme_constant_override("margin_bottom", int(ins.z + 12))

func _refresh() -> void:
	var r := await Api.get_json("/v1/me")
	if not r.ok:
		_status.text = r.error
		_render_side()
		return
	Game.characters = r.data.characters
	_selected = 0 if not Game.characters.is_empty() else -1
	_render_roster()
	_select(_selected)
	if Cfg.autotest:
		await get_tree().create_timer(2.5).timeout
		await Cfg.shot("01_select")
		if not Game.characters.is_empty() and Cfg.shot_dir != "":
			var credits: Node = Popups.open(preload("res://scenes/ui/credits_panel.gd").new())
			await get_tree().create_timer(2.0).timeout
			await Cfg.shot("00_credits")
			credits.queue_free()
		if Game.characters.is_empty():
			_name.text = "auto%d" % (randi() % 100000)
			await _create()
		else:
			_play(Game.characters[0])

func _render_roster() -> void:
	for c in _roster.get_children():
		c.queue_free()
	for i in Game.characters.size():
		_roster.add_child(_card(i))
	if Game.characters.size() < MAX_CHARACTERS:
		var b := UiKit.button("New hero", func(): _select(-1), 56, "orange" if _selected == -1 else "gray", "star")
		_roster.add_child(b)

func _card(i: int) -> Control:
	var ch: Dictionary = Game.characters[i]
	var b := Button.new()
	b.custom_minimum_size = Vector2(0, 76)
	b.focus_mode = Control.FOCUS_NONE
	var box := UiKit.panel_box(8)
	if i == _selected:
		box.rim_top = UiKit.GOLD
		box.rim_bottom = UiKit.GOLD.darkened(0.5)
	for st in ["normal", "hover", "pressed"]:
		b.add_theme_stylebox_override(st, box)
	b.add_theme_stylebox_override("focus", StyleBoxEmpty.new())
	var row := HBoxContainer.new()
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	UiKit.full_rect(row)
	row.offset_left = 8
	b.add_child(row)
	var p := Portrait.new(60)
	p.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	p.show_recipe(ch.appearance)
	row.add_child(p)
	var col := VBoxContainer.new()
	col.alignment = BoxContainer.ALIGNMENT_CENTER
	col.add_theme_constant_override("separation", -2)
	col.add_child(UiKit.label(str(ch.name), 20, UiKit.TEXT))
	var cls := str(ch.get("class", ""))
	col.add_child(UiKit.label("Lv. %d  %s" % [int(ch.level), cls.capitalize() if cls != "" else "Wanderer"], 14, UiKit.GOLD, true, 4))
	row.add_child(col)
	b.pressed.connect(func():
		Telegram.haptic("selection")
		_select(i))
	return b

func _select(i: int) -> void:
	_selected = i
	_render_roster()
	if i >= 0:
		var ch: Dictionary = Game.characters[i]
		_preview.show_recipe(ch.appearance)
		_stage_name.text = str(ch.name)
	else:
		_stage_name.text = "New hero"
		if _recipe.is_empty():
			_reroll()
		else:
			_preview.show_recipe(_recipe)
	_render_side()

func _render_side() -> void:
	for c in _side.get_children():
		if c != _status:
			c.queue_free()
	if _status.get_parent():
		_side.remove_child(_status)
	if _selected >= 0:
		var ch: Dictionary = Game.characters[_selected]
		_side.add_child(UiKit.title(str(ch.name), 30))
		var info := UiKit.inset(12)
		var ic := VBoxContainer.new()
		info.add_child(ic)
		var cls := str(ch.get("class", ""))
		ic.add_child(UiKit.stat_row("Level", str(int(ch.level))))
		ic.add_child(UiKit.stat_row("Class", cls.capitalize() if cls != "" else "Wanderer"))
		ic.add_child(UiKit.stat_row("World", str(int(ch.get("world_id", 1)))))
		_side.add_child(info)
		_side.add_child(UiKit.vspacer())
		_side.add_child(_status)
		var play_btn := UiKit.button("PLAY", func(): _play(ch), 72, "orange", "attack")
		play_btn.add_theme_font_size_override("font_size", 30)
		_side.add_child(play_btn)
	else:
		_side.add_child(UiKit.title("New hero", 30))
		_side.add_child(UiKit.para("Your look is woven from a random seed. Your hidden Root and talents are too - you discover them by playing.", 15, UiKit.MUTED))
		var bodies := HBoxContainer.new()
		for b in ["male", "female"]:
			var btn := UiKit.button(b.capitalize(), func():
				_body = b
				_reroll(false)
				_render_side(), 44, "orange" if _body == b else "gray")
			btn.size_flags_horizontal = Control.SIZE_EXPAND_FILL
			bodies.add_child(btn)
		_side.add_child(bodies)
		_side.add_child(UiKit.button("Randomise look", func(): _reroll(), 46, "blue", "sparkle"))
		_name = LineEdit.new()
		_name.placeholder_text = "Name (3-16 letters)"
		_name.max_length = 16
		_name.custom_minimum_size = Vector2(0, 50)
		_side.add_child(_name)
		_side.add_child(UiKit.vspacer())
		_side.add_child(_status)
		_side.add_child(UiKit.button("Create", _create, 64, "green", "star"))

func _reroll(new_seed := true) -> void:
	if new_seed:
		_seed = randi()
	var r := await Api.get_json("/v1/sprites/random?seed=%d&body=%s" % [_seed, _body])
	if r.ok:
		_recipe = r.data.recipe
		if _selected == -1:
			_preview.show_recipe(_recipe)

func _create() -> void:
	if Game.characters.size() >= MAX_CHARACTERS:
		_status.text = "You can have at most %d characters." % MAX_CHARACTERS
		return
	_status.text = "Creating..."
	var body := {"name": _name.text}
	if not _recipe.is_empty():
		body["appearance"] = _recipe
	var r := await Api.post_json("/v1/characters", body)
	if not r.ok:
		_status.text = r.error
		Telegram.haptic("error")
		return
	_status.text = ""
	_recipe = {}
	Telegram.haptic("success")
	_refresh()

func _play(ch: Dictionary) -> void:
	_status.text = "Entering the world..."
	var r := await Api.post_json("/v1/session/character", {"character_id": ch.id})
	if not r.ok:
		_status.text = r.error
		return
	Api.token = r.data.token
	Game.set_character(r.data.character)
	Game.world = r.data.world
	Game.position = r.data.position
	Game.realtime = r.data.realtime
	Game.tileset_url = r.data.tileset
	play.emit()
