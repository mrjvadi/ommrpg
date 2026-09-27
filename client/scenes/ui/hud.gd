class_name Hud
extends Control
## In-game HUD. Every element lives in a HudItem whose position, scale,
## visibility (and direction, for the menu) come from HudLayout, so players
## arrange their own HUD in the layout editor: drag to move (with snapping
## and alignment guides), drag the corner handle or use -/+ to resize, hide
## elements, flip the menu between a row and a column, and keep up to three
## layouts.

signal attack_pressed
signal interact_pressed
signal menu(action: String)

const MENU := [["hero", "Hero", "character"], ["bag", "Bag", "inventory"], ["trade", "Trade", "trade"],
	["chronicle", "Chronicle", "history"], ["layout", "Layout", "layout"], ["settings", "Settings", "settings"]]

var items := {} # id -> HudItem
var joystick: VirtualJoystick
var attack_btn: VirtualJoystick.TouchButton
var interact_btn: VirtualJoystick.TouchButton
var status: StatusPlate
var hp_bar: ProgressBar
var xp_bar: ProgressBar
var level_label: Label
var name_label: Label
var class_label: Label
var portrait: Portrait
var gold_pill: UiKit.Pill
var essence_pill: UiKit.Pill
var ton_pill: UiKit.Pill
var minimap: MinimapFrame
var log_box: VBoxContainer
var menu_box: BoxContainer
var menu_buttons := {} # action -> Medallion
var target_panel: PanelContainer
var target_name: Label
var target_bar: ProgressBar
var editing := false
var snapping := true
var guides: Array = [] # [axis ("x"/"y"), position] shown while dragging
var _editor: Control
var _editor_label: Label
var _selected := ""
var _guide_layer: Control

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
	Popups.hud_log_active = true
	tree_exited.connect(func(): Popups.hud_log_active = false)
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
	# character plate: portrait orb, name, health and experience
	status = StatusPlate.new(0.42)
	status.portrait_pressed.connect(func(): menu.emit("character"))
	hp_bar = status.hp_bar
	xp_bar = status.xp_bar
	level_label = status.level_label
	name_label = status.name_label
	class_label = status.class_label
	portrait = status.portrait
	_item("status", status)

	# currencies
	var wallet := HBoxContainer.new()
	wallet.add_theme_constant_override("separation", 6)
	gold_pill = UiKit.pill("gold", "gold", func(): menu.emit("trade"))
	essence_pill = UiKit.pill("essence", "purple")
	ton_pill = UiKit.pill("ton", "ton", func(): menu.emit("wallet"))
	for p in [gold_pill, essence_pill, ton_pill]:
		wallet.add_child(p)
	_item("wallet", wallet)

	# round minimap with the zone plaque
	minimap = MinimapFrame.new(0.42)
	_item("minimap", minimap)

	# menu medallions
	menu_box = HBoxContainer.new()
	menu_box.add_theme_constant_override("separation", 10)
	_item("menu", menu_box)
	_fill_menu()

	joystick = VirtualJoystick.new()
	_item("joystick", joystick)

	attack_btn = VirtualJoystick.TouchButton.new("ATK", 112, Color("c9352b"), "attack")
	attack_btn.pressed.connect(func(): attack_pressed.emit())
	_item("attack", attack_btn)
	interact_btn = VirtualJoystick.TouchButton.new("USE", 76, Color("2e7fc9"), "use")
	interact_btn.pressed.connect(func(): interact_pressed.emit())
	_item("use", interact_btn)

	# target frame (shown while a monster is targeted)
	target_panel = PanelContainer.new()
	target_panel.add_theme_stylebox_override("panel", UiKit.tooltip_box(4))
	var tcol := VBoxContainer.new()
	tcol.add_theme_constant_override("separation", 2)
	target_panel.add_child(tcol)
	target_name = UiKit.label("", 16, UiKit.TEXT, true, 3)
	target_name.add_theme_font_override("font", UiKit.FONT_TITLE)
	target_name.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	tcol.add_child(target_name)
	target_bar = UiKit.value_bar(Color("c9352b"), 20)
	target_bar.custom_minimum_size.x = 230
	tcol.add_child(target_bar)
	target_panel.visible = false
	_item("target", target_panel)

	log_box = VBoxContainer.new()
	log_box.custom_minimum_size = Vector2(420, 10)
	log_box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	log_box.add_theme_constant_override("separation", 2)
	_item("log", log_box)

	_guide_layer = Guides.new()
	_guide_layer.hud = self
	add_child(UiKit.full_rect(_guide_layer))

func _fill_menu() -> void:
	for c in menu_box.get_children():
		c.queue_free()
	menu_buttons.clear()
	for m in MENU:
		var b := Medallion.new(m[0], m[1], 58)
		var action: String = m[2]
		b.pressed.connect(func():
			Telegram.haptic("light")
			menu.emit(action))
		menu_box.add_child(b)
		menu_buttons[action] = b

## Row or column, from the layout.
func _apply_menu_dir(d: String) -> void:
	var want_col := d == "col"
	if (menu_box is VBoxContainer) == want_col:
		return
	var nb: BoxContainer = VBoxContainer.new() if want_col else HBoxContainer.new()
	nb.add_theme_constant_override("separation", 10 if not want_col else 4)
	var it: HudItem = items["menu"]
	var badges := {}
	for k in menu_buttons.keys():
		badges[k] = menu_buttons[k]._badge.visible
	it.remove_child(menu_box)
	menu_box.queue_free()
	menu_box = nb
	it.add_child(nb)
	_fill_menu()
	for k in badges.keys():
		set_badge(k, badges[k])
	if editing:
		it._apply_filter(it, true)

func set_badge(action: String, on: bool) -> void:
	if menu_buttons.has(action):
		menu_buttons[action].set_badge(on)

func safe_rect() -> Rect2:
	var vs := get_viewport_rect().size
	var ins := Telegram.insets_for(get_viewport())
	var margin := 8.0
	return Rect2(ins.w + margin, ins.x + margin, vs.x - ins.w - ins.y - margin * 2, vs.y - ins.x - ins.z - margin * 2)

func apply_layout() -> void:
	if not is_inside_tree():
		return
	_apply_menu_dir(str(HudLayout.get_entry("menu").get("d", "row")))
	var sr := safe_rect()
	for id in items.keys():
		var it: HudItem = items[id]
		it.place(HudLayout.get_entry(id), sr)
	_update_editor_label()

func _on_character() -> void:
	var c := Game.character
	if c.is_empty():
		return
	var cls := str(c.get("class", ""))
	name_label.text = str(c.name)
	class_label.text = cls.capitalize() if cls != "" else "Wanderer"
	level_label.text = str(int(c.level))
	UiKit.set_bar(xp_bar, int(c.get("xp", 0)), max(1, int(c.get("xp_to_next", 1))), "%s / %s" % [UiKit.short(c.get("xp", 0)), UiKit.short(c.get("xp_to_next", 1))])
	portrait.show_recipe(c.appearance)
	set_badge("character", int(c.get("free_points", 0)) > 0)

func _on_wallet() -> void:
	gold_pill.set_value(UiKit.short(Game.wallet.get("gold", 0)))
	essence_pill.set_value(UiKit.short(Game.wallet.get("essence", 0)))
	ton_pill.set_value(("%.2f" % (float(Game.ton_balance) / 1e9)).rstrip("0").rstrip("."))

func _on_vitals() -> void:
	UiKit.set_bar(hp_bar, Game.hp, Game.max_hp)

func set_minimap(img: Image) -> void:
	minimap.set_map(ImageTexture.create_from_image(img))

func set_zone(text: String) -> void:
	minimap.set_zone(text)

## Target frame: pass an empty name to hide it.
func set_target(tname: String, hp: int, max_hp: int, color := UiKit.TEXT) -> void:
	target_panel.visible = tname != "" or editing
	if tname == "":
		return
	target_name.text = tname
	target_name.add_theme_color_override("font_color", color)
	UiKit.set_bar(target_bar, hp, max_hp)

func toast(text: String, color := Color.WHITE) -> void:
	if Popups.has_open():
		return # Popups shows it above the open window
	var l := UiKit.label(text, 17, color, true, 6)
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.custom_minimum_size.x = 420
	log_box.add_child(l)
	while log_box.get_child_count() > 5:
		log_box.get_child(0).free()
	var tw := l.create_tween()
	tw.tween_interval(3.5)
	tw.tween_property(l, "modulate:a", 0.0, 0.8)
	tw.tween_callback(l.queue_free)

# ---------------------------------------------------------------- layout editor

func start_editing() -> void:
	if editing:
		return
	editing = true
	joystick.enabled = false
	attack_btn.enabled = false
	interact_btn.enabled = false
	target_panel.visible = true
	if target_name.text == "":
		target_name.text = "Target"
		UiKit.set_bar(target_bar, 70, 100)
	if log_box.get_child_count() == 0:
		var sample := UiKit.label("Messages appear here", 17, UiKit.MUTED, true, 5)
		sample.name = "Sample"
		log_box.add_child(sample)
	for it in items.values():
		it.set_editing(true)
	_editor = _build_editor()
	add_child(_editor)
	select("")
	apply_layout()

func stop_editing(save: bool) -> void:
	editing = false
	joystick.enabled = true
	attack_btn.enabled = true
	interact_btn.enabled = true
	target_panel.visible = false
	var sample := log_box.get_node_or_null("Sample")
	if sample:
		sample.queue_free()
	for it in items.values():
		it.set_editing(false)
	guides.clear()
	_guide_layer.queue_redraw()
	if _editor:
		_editor.queue_free()
		_editor = null
	if save:
		await HudLayout.save()
		Game.toast("Layout %d saved" % HudLayout.active, UiKit.GOOD)
	else:
		HudLayout.reload()
		apply_layout()

func select(id: String) -> void:
	_selected = id
	for k in items.keys():
		items[k].selected = k == id
		items[k].queue_redraw()
	_update_editor_label()

func _update_editor_label() -> void:
	if not is_instance_valid(_editor_label):
		return
	if _selected == "":
		_editor_label.text = "Layout %d - drag any element" % HudLayout.active
	else:
		var e := HudLayout.get_entry(_selected)
		_editor_label.text = "%s  %d%%%s" % [HudLayout.NAMES.get(_selected, _selected), int(round(float(e.s) * 100)), "  (hidden)" if not e.v else ""]

func _build_editor() -> Control:
	var wrap := Control.new()
	UiKit.full_rect(wrap)
	wrap.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var p := UiKit.panel(10)
	wrap.add_child(p)
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 6)
	p.add_child(col)
	var top := HBoxContainer.new()
	col.add_child(top)
	_editor_label = UiKit.label("", 17, UiKit.GOLD, true, 5)
	_editor_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_editor_label.custom_minimum_size.x = 250
	top.add_child(_editor_label)
	for i in range(1, HudLayout.SLOTS + 1):
		var b := UiKit.button(str(i), func():
			HudLayout.use_slot(i)
			_rebuild_editor(), 38, "orange" if HudLayout.active == i else "gray")
		b.custom_minimum_size.x = 42
		top.add_child(b)
	var row := HBoxContainer.new()
	col.add_child(row)
	var small := func(t: String, cb: Callable, kind := "gray", w := 0.0) -> Button:
		var b := UiKit.button(t, cb, 40, kind)
		b.custom_minimum_size.x = w
		row.add_child(b)
		return b
	small.call("-", func(): _scale_selected(-0.1), "gray", 44)
	small.call("+", func(): _scale_selected(0.1), "gray", 44)
	small.call("Hide/Show", _toggle_selected)
	small.call("Row/Column", _flip_menu)
	var snap := CheckButton.new()
	snap.text = "Snap"
	snap.button_pressed = snapping
	snap.toggled.connect(func(on): snapping = on)
	row.add_child(snap)
	var row2 := HBoxContainer.new()
	col.add_child(row2)
	var b1 := UiKit.button("Reset", func():
		if await Popups.confirm("Reset layout %d?" % HudLayout.active, "Every element goes back to its default place.", "Reset", "red"):
			HudLayout.reset()
			apply_layout(), 42, "red")
	b1.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row2.add_child(b1)
	var b2 := UiKit.button("Cancel", func(): stop_editing(false), 42, "gray")
	b2.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row2.add_child(b2)
	var b3 := UiKit.button("Save", func(): stop_editing(true), 42, "green")
	b3.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row2.add_child(b3)
	var move := UiKit.button("Move bar", func(): _toggle_editor_side(p), 42, "blue")
	row2.add_child(move)
	p.set_meta("top", true)
	p.resized.connect(func(): _place_editor(p))
	_place_editor.call_deferred(p)
	return wrap

func _rebuild_editor() -> void:
	if _editor:
		_editor.queue_free()
	_editor = _build_editor()
	add_child(_editor)
	apply_layout()

func _toggle_editor_side(p: Control) -> void:
	p.set_meta("top", not bool(p.get_meta("top")))
	_place_editor(p)

func _place_editor(p: Control) -> void:
	var sr := safe_rect()
	var sz := p.get_combined_minimum_size()
	p.size = sz
	var y := sr.position.y + sr.size.y * 0.36 if bool(p.get_meta("top")) else sr.end.y - sz.y - sr.size.y * 0.2
	p.position = Vector2(sr.position.x + (sr.size.x - sz.x) / 2, y)

func _scale_selected(d: float) -> void:
	if _selected == "":
		Game.toast("Select an element first", UiKit.MUTED)
		return
	var e := HudLayout.get_entry(_selected)
	e.s = clampf(snappedf(float(e.s) + d, 0.05), 0.5, 2.0)
	apply_layout()

func _toggle_selected() -> void:
	if _selected == "":
		Game.toast("Select an element first", UiKit.MUTED)
		return
	var e := HudLayout.get_entry(_selected)
	e.v = not bool(e.v)
	apply_layout()

func _flip_menu() -> void:
	var e := HudLayout.get_entry("menu")
	e.d = "col" if str(e.get("d", "row")) == "row" else "row"
	select("menu")
	apply_layout()

## Snaps a dragged element's centre to the screen centre lines and to the
## edges / centres of other elements, remembering guides to draw.
func snap_center(id: String, center: Vector2, real: Vector2, sr: Rect2) -> Vector2:
	guides.clear()
	if not snapping:
		return center
	var th := 10.0
	var xs := [sr.position.x + real.x / 2, sr.get_center().x, sr.end.x - real.x / 2]
	var ys := [sr.position.y + real.y / 2, sr.get_center().y, sr.end.y - real.y / 2]
	var guide_x := [sr.position.x, sr.get_center().x, sr.end.x]
	var guide_y := [sr.position.y, sr.get_center().y, sr.end.y]
	for k in items.keys():
		if k == id or not items[k].visible:
			continue
		var o: HudItem = items[k]
		var r := Rect2(o.position, o.size * o.scale)
		xs += [r.get_center().x, r.position.x + real.x / 2, r.end.x - real.x / 2]
		guide_x += [r.get_center().x, r.position.x, r.end.x]
		ys += [r.get_center().y, r.position.y + real.y / 2, r.end.y - real.y / 2]
		guide_y += [r.get_center().y, r.position.y, r.end.y]
	var out := center
	var best := th
	for i in xs.size():
		if absf(xs[i] - center.x) < best:
			best = absf(xs[i] - center.x)
			out.x = xs[i]
			guides = guides.filter(func(g): return g[0] != "x")
			guides.append(["x", guide_x[i]])
	best = th
	for i in ys.size():
		if absf(ys[i] - center.y) < best:
			best = absf(ys[i] - center.y)
			out.y = ys[i]
			guides = guides.filter(func(g): return g[0] != "y")
			guides.append(["y", guide_y[i]])
	_guide_layer.queue_redraw()
	return out

func end_drag() -> void:
	guides.clear()
	_guide_layer.queue_redraw()


class HudItem extends Control:
	## Wraps one HUD element; positions it from the layout entry and lets the
	## player drag it (or its corner handle) in edit mode.
	var id := ""
	var hud: Hud
	var editing := false
	var selected := false
	var _mode := "" # "move" or "scale"
	var _grab := Vector2.ZERO
	var _scale_from := 1.0
	var _dist_from := 1.0

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

	func _handle_pos() -> Vector2:
		return size + Vector2(4, 4)

	func _gui_input(e: InputEvent) -> void:
		if not editing:
			return
		if e is InputEventMouseButton and e.button_index == MOUSE_BUTTON_LEFT:
			if e.pressed:
				var entry := HudLayout.get_entry(id)
				if selected and e.position.distance_to(_handle_pos()) < 26.0:
					_mode = "scale"
					_scale_from = float(entry.s)
					_dist_from = maxf(20.0, (get_global_mouse_position() - global_position).length())
				else:
					_mode = "move"
					_grab = e.position * scale
				hud.select(id)
				Telegram.haptic("selection")
			else:
				_mode = ""
				hud.end_drag()
			accept_event()
		elif e is InputEventMouseMotion and _mode != "":
			var sr := hud.safe_rect()
			var entry := HudLayout.get_entry(id)
			if _mode == "scale":
				var d := (get_global_mouse_position() - global_position).length()
				entry.s = clampf(snappedf(_scale_from * d / _dist_from, 0.05), 0.5, 2.0)
				hud._update_editor_label()
			else:
				var real := size * scale
				var top_left := get_global_mouse_position() - _grab
				var center := hud.snap_center(id, top_left + real / 2, real, sr)
				entry.x = clampf((center.x - sr.position.x) / sr.size.x, 0.0, 1.0)
				entry.y = clampf((center.y - sr.position.y) / sr.size.y, 0.0, 1.0)
			place(entry, sr)
			accept_event()

	func _draw() -> void:
		if not editing:
			return
		var col := Color(1, 0.83, 0.35) if selected else Color(1, 1, 1, 0.55)
		var r := Rect2(Vector2(-4, -4), size + Vector2(8, 8))
		draw_rect(r, Color(col, 0.1))
		var w := 2.5 / maxf(0.5, scale.x)
		# dashed outline
		var step := 10.0
		var pts := [[r.position, Vector2(r.end.x, r.position.y)], [Vector2(r.end.x, r.position.y), r.end], [r.end, Vector2(r.position.x, r.end.y)], [Vector2(r.position.x, r.end.y), r.position]]
		for seg in pts:
			var a: Vector2 = seg[0]
			var b: Vector2 = seg[1]
			var n := int(a.distance_to(b) / step)
			for i in range(0, n, 2):
				draw_line(a.lerp(b, float(i) / n), a.lerp(b, float(i + 1) / n), col, w)
		if selected:
			var h := _handle_pos()
			var hr := 11.0 / maxf(0.5, scale.x)
			draw_circle(h, hr + 2, UiKit.OUTLINE)
			draw_circle(h, hr, UiKit.GOLD)
			draw_line(h + Vector2(-hr * 0.5, hr * 0.5), h + Vector2(hr * 0.5, -hr * 0.5), UiKit.OUTLINE, w)


class Guides extends Control:
	## Alignment guides drawn while dragging, plus a faint grid in edit mode.
	var hud: Hud

	func _init() -> void:
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		if hud == null or not hud.editing:
			return
		var sr := hud.safe_rect()
		var grid := Color(1, 1, 1, 0.05)
		var x := sr.position.x
		while x <= sr.end.x:
			draw_line(Vector2(x, sr.position.y), Vector2(x, sr.end.y), grid, 1.0)
			x += sr.size.x / 24.0
		var y := sr.position.y
		while y <= sr.end.y:
			draw_line(Vector2(sr.position.x, y), Vector2(sr.end.x, y), grid, 1.0)
			y += sr.size.y / 12.0
		draw_rect(sr, Color(1, 0.83, 0.35, 0.35), false, 2.0)
		for g in hud.guides:
			if g[0] == "x":
				draw_line(Vector2(g[1], sr.position.y), Vector2(g[1], sr.end.y), Color("4de0ff"), 2.0)
			else:
				draw_line(Vector2(sr.position.x, g[1]), Vector2(sr.end.x, g[1]), Color("4de0ff"), 2.0)


## True when a screen point is over a visible HUD element.
func is_over_ui(p: Vector2) -> bool:
	for it in items.values():
		if it.visible and it.id != "log" and it.id != "target" and Rect2(it.global_position, it.size * it.scale).grow(6).has_point(p):
			return true
	return false
