class_name Hud
extends Control
## In-game HUD. Every element lives in a HudItem whose position and scale
## come from HudLayout, so players can rearrange the HUD (edit mode) and the
## layout adapts to portrait/landscape and Telegram's safe areas.

signal attack_pressed
signal interact_pressed
signal menu(action: String)

var items := {} # id -> HudItem
var joystick: VirtualJoystick
var attack_btn: VirtualJoystick.TouchButton
var interact_btn: VirtualJoystick.TouchButton
var hp_bar: ProgressBar
var xp_bar: ProgressBar
var hp_label: Label
var level_label: Label
var gold_label: Label
var essence_label: Label
var minimap: TextureRect
var log_box: VBoxContainer
var editing := false
var _editor: Control
var _selected := ""

func _ready() -> void:
	UiKit.full_rect(self)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_build()
	HudLayout.changed.connect(apply_layout)
	get_viewport().size_changed.connect(apply_layout)
	Telegram.insets_changed.connect(apply_layout)
	Game.character_changed.connect(_on_character)
	Game.wallet_changed.connect(_on_wallet)
	Game.vitals_changed.connect(_on_vitals)
	Game.notify.connect(toast)
	_on_character()
	_on_wallet()
	_on_vitals()
	apply_layout.call_deferred()

func _item(id: String, content: Control) -> Control:
	var it := HudItem.new()
	it.id = id
	it.hud = self
	it.add_child(content)
	add_child(it)
	items[id] = it
	return content

func _build() -> void:
	# vitals: level, hp and xp bars
	var vp := UiKit.panel()
	vp.custom_minimum_size = Vector2(230, 0)
	vp.add_theme_stylebox_override("panel", UiKit.box(Color(0.06, 0.06, 0.1, 0.72), 8, 1, Color(1, 1, 1, 0.12), 8))
	var vb := VBoxContainer.new()
	vb.add_theme_constant_override("separation", 3)
	vp.add_child(vb)
	level_label = UiKit.label("", 15, UiKit.ACCENT, true)
	vb.add_child(level_label)
	hp_bar = UiKit.bar(UiKit.DANGER, 16)
	vb.add_child(hp_bar)
	hp_label = UiKit.label("", 11, UiKit.TEXT)
	hp_label.position = Vector2(6, -1)
	hp_bar.add_child(hp_label)
	xp_bar = UiKit.bar(Color("5aa0ff"), 7)
	vb.add_child(xp_bar)
	_item("vitals", vp)

	var wp := UiKit.panel()
	wp.add_theme_stylebox_override("panel", UiKit.box(Color(0.06, 0.06, 0.1, 0.72), 8, 1, Color(1, 1, 1, 0.12), 8))
	var wb := HBoxContainer.new()
	wp.add_child(wb)
	gold_label = UiKit.label("0", 15, Color("ffd24a"), true)
	essence_label = UiKit.label("0", 15, Color("a88cff"), true)
	wb.add_child(UiKit.label("Gold", 12, UiKit.MUTED))
	wb.add_child(gold_label)
	wb.add_child(UiKit.label("Essence", 12, UiKit.MUTED))
	wb.add_child(essence_label)
	_item("wallet", wp)

	minimap = TextureRect.new()
	minimap.custom_minimum_size = Vector2(120, 120)
	minimap.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	minimap.stretch_mode = TextureRect.STRETCH_SCALE
	minimap.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	var mm_frame := PanelContainer.new()
	mm_frame.add_theme_stylebox_override("panel", UiKit.box(Color(0, 0, 0, 0.5), 6, 2, Color(1, 1, 1, 0.2), 2))
	mm_frame.add_child(minimap)
	var dot := ColorRect.new()
	dot.color = Color.WHITE
	dot.size = Vector2(4, 4)
	dot.position = Vector2(60, 60)
	minimap.add_child(dot)
	_item("minimap", mm_frame)

	var menu_row := HBoxContainer.new()
	menu_row.add_theme_constant_override("separation", 6)
	for pair in [["Bag", "inventory"], ["Hero", "character"], ["Chronicle", "history"], ["Layout", "layout"], ["Exit", "exit"]]:
		var b := UiKit.button(pair[0], func(): menu.emit(pair[1]), 40)
		b.add_theme_font_size_override("font_size", 15)
		b.custom_minimum_size.x = 72
		menu_row.add_child(b)
	_item("menu", menu_row)

	joystick = VirtualJoystick.new()
	_item("joystick", joystick)

	var actions := Control.new()
	actions.custom_minimum_size = Vector2(190, 170)
	actions.mouse_filter = Control.MOUSE_FILTER_IGNORE
	attack_btn = VirtualJoystick.TouchButton.new("ATK", 104, Color("c0392b"))
	attack_btn.position = Vector2(80, 60)
	attack_btn.pressed.connect(func(): attack_pressed.emit())
	actions.add_child(attack_btn)
	interact_btn = VirtualJoystick.TouchButton.new("USE", 72, Color("2e86c1"))
	interact_btn.position = Vector2(4, 4)
	interact_btn.pressed.connect(func(): interact_pressed.emit())
	actions.add_child(interact_btn)
	_item("actions", actions)

	log_box = VBoxContainer.new()
	log_box.custom_minimum_size = Vector2(360, 10)
	log_box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_item("log", log_box)

func safe_rect() -> Rect2:
	var vs := get_viewport_rect().size
	var ins := Telegram.insets_for(get_viewport())
	var margin := 6.0
	return Rect2(ins.w + margin, ins.x + margin, vs.x - ins.w - ins.y - margin * 2, vs.y - ins.x - ins.z - margin * 2)

func orientation() -> String:
	return HudLayout.orientation(get_viewport_rect().size)

func apply_layout() -> void:
	var orient := orientation()
	var sr := safe_rect()
	for id in items.keys():
		var it: HudItem = items[id]
		it.place(HudLayout.get_entry(orient, id), sr)

func _on_character() -> void:
	var c := Game.character
	if c.is_empty():
		return
	var cls := str(c.get("class", ""))
	level_label.text = "%s  Lv %d%s" % [str(c.name), int(c.level), ("  " + cls.capitalize()) if cls != "" else ""]
	xp_bar.max_value = max(1, int(c.get("xp_to_next", 1)))
	xp_bar.value = int(c.get("xp", 0))

func _on_wallet() -> void:
	gold_label.text = str(int(Game.wallet.get("gold", 0)))
	essence_label.text = str(int(Game.wallet.get("essence", 0)))

func _on_vitals() -> void:
	hp_bar.max_value = Game.max_hp
	hp_bar.value = Game.hp
	hp_label.text = "%d / %d" % [Game.hp, Game.max_hp]

func set_minimap(img: Image) -> void:
	minimap.texture = ImageTexture.create_from_image(img)

func toast(text: String, color := Color.WHITE) -> void:
	var l := UiKit.label(text, 15, color, true)
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.add_theme_color_override("font_outline_color", Color.BLACK)
	l.add_theme_constant_override("outline_size", 6)
	log_box.add_child(l)
	while log_box.get_child_count() > 5:
		log_box.get_child(0).free()
	var tw := l.create_tween()
	tw.tween_interval(3.5)
	tw.tween_property(l, "modulate:a", 0.0, 0.8)
	tw.tween_callback(l.queue_free)

# ---------------------------------------------------------------- layout editor

func start_editing() -> void:
	editing = true
	joystick.enabled = false
	attack_btn.enabled = false
	interact_btn.enabled = false
	for it in items.values():
		it.set_editing(true)
	_editor = _build_editor()
	add_child(_editor)

func stop_editing(save: bool) -> void:
	editing = false
	joystick.enabled = true
	attack_btn.enabled = true
	interact_btn.enabled = true
	for it in items.values():
		it.set_editing(false)
	if _editor:
		_editor.queue_free()
		_editor = null
	if save:
		HudLayout.save()
	else:
		HudLayout.reload()
		apply_layout()

func select(id: String) -> void:
	_selected = id
	for k in items.keys():
		items[k].selected = k == id
		items[k].queue_redraw()

func _build_editor() -> Control:
	var p := UiKit.panel()
	var col := VBoxContainer.new()
	p.add_child(col)
	col.add_child(UiKit.para("Drag elements to move them. Select one to resize or hide it.", 14, UiKit.MUTED))
	var row := HBoxContainer.new()
	col.add_child(row)
	row.add_child(UiKit.button("Smaller", func(): _scale_selected(-0.1), 40))
	row.add_child(UiKit.button("Bigger", func(): _scale_selected(0.1), 40))
	row.add_child(UiKit.button("Hide/Show", _toggle_selected, 40))
	var row2 := HBoxContainer.new()
	col.add_child(row2)
	row2.add_child(UiKit.button("Save", func(): stop_editing(true), 44))
	row2.add_child(UiKit.button("Reset", func():
		HudLayout.reset(orientation())
		apply_layout(), 44))
	row2.add_child(UiKit.button("Cancel", func(): stop_editing(false), 44))
	p.custom_minimum_size = Vector2(360, 0)
	var center := CenterContainer.new()
	UiKit.full_rect(center)
	center.mouse_filter = Control.MOUSE_FILTER_IGNORE
	center.add_child(p)
	return center

func _scale_selected(d: float) -> void:
	if _selected == "":
		return
	var e := HudLayout.get_entry(orientation(), _selected)
	e.s = clampf(float(e.s) + d, 0.5, 2.0)
	apply_layout()

func _toggle_selected() -> void:
	if _selected == "":
		return
	var e := HudLayout.get_entry(orientation(), _selected)
	e.v = not bool(e.v)
	apply_layout()


class HudItem extends Control:
	## Wraps one HUD element; positions it from the layout entry and lets the
	## player drag it in edit mode.
	var id := ""
	var hud: Hud
	var editing := false
	var selected := false
	var _dragging := false
	var _grab := Vector2.ZERO

	func _init() -> void:
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func content() -> Control:
		return get_child(0) as Control

	func place(entry: Dictionary, sr: Rect2) -> void:
		var c := content()
		if c == null:
			return
		var s := float(entry.s)
		size = c.get_combined_minimum_size()
		c.size = size
		scale = Vector2(s, s)
		var real := size * s
		var center := sr.position + Vector2(float(entry.x), float(entry.y)) * sr.size
		var pos := center - real / 2
		pos.x = clampf(pos.x, sr.position.x, max(sr.position.x, sr.end.x - real.x))
		pos.y = clampf(pos.y, sr.position.y, max(sr.position.y, sr.end.y - real.y))
		position = pos
		visible = bool(entry.v) or editing
		modulate.a = 1.0 if bool(entry.v) else 0.35
		queue_redraw()

	func set_editing(on: bool) -> void:
		editing = on
		selected = false
		mouse_filter = Control.MOUSE_FILTER_STOP if on else Control.MOUSE_FILTER_IGNORE
		_apply_filter(self, on)
		visible = true
		queue_redraw()

	## In edit mode the wrapped widgets must ignore input so drags reach us.
	func _apply_filter(n: Node, on: bool) -> void:
		for ch in n.get_children():
			if ch is Control:
				if not ch.has_meta("orig_filter"):
					ch.set_meta("orig_filter", ch.mouse_filter)
				ch.mouse_filter = Control.MOUSE_FILTER_IGNORE if on else int(ch.get_meta("orig_filter"))
			_apply_filter(ch, on)

	func _gui_input(e: InputEvent) -> void:
		if not editing:
			return
		if e is InputEventMouseButton and e.button_index == MOUSE_BUTTON_LEFT:
			if e.pressed:
				_dragging = true
				_grab = e.position * scale
				hud.select(id)
			else:
				_dragging = false
			accept_event()
		elif e is InputEventMouseMotion and _dragging:
			var sr := hud.safe_rect()
			var real := size * scale
			var top_left := get_global_mouse_position() - _grab
			var center := top_left + real / 2
			var entry := HudLayout.get_entry(hud.orientation(), id)
			entry.x = clampf((center.x - sr.position.x) / sr.size.x, 0.0, 1.0)
			entry.y = clampf((center.y - sr.position.y) / sr.size.y, 0.0, 1.0)
			place(entry, sr)
			accept_event()

	func _draw() -> void:
		if not editing:
			return
		var col := Color(1, 0.85, 0.3) if selected else Color(1, 1, 1, 0.6)
		draw_rect(Rect2(Vector2(-3, -3), size + Vector2(6, 6)), Color(col, 0.12))
		draw_rect(Rect2(Vector2(-3, -3), size + Vector2(6, 6)), col, false, 2.0)

## True when a screen point is over a visible HUD element.
func is_over_ui(p: Vector2) -> bool:
	for it in items.values():
		if it.visible and it.id != "log" and Rect2(it.global_position, it.size * it.scale).grow(6).has_point(p):
			return true
	return false
