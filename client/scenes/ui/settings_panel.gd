class_name SettingsPanel
extends Modal
## Game settings: screen (full screen, auto-rotate), HUD layouts, haptics,
## home-screen shortcut, credits and leaving the world.

signal edit_layout
signal exit_world
signal rotation_changed

var in_world := false

func _init() -> void:
	super("Settings")
	max_size = Vector2(800, 560)
	fit_content = true

func _ready() -> void:
	super()
	_render()

func _render() -> void:
	clear_body()
	var cols := HBoxContainer.new()
	cols.add_theme_constant_override("separation", 16)
	body.add_child(cols)
	var left := VBoxContainer.new()
	left.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	cols.add_child(left)
	var right := VBoxContainer.new()
	right.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	cols.add_child(right)

	left.add_child(UiKit.heading("Screen", 20))
	if Telegram.at_least("8.0"):
		var fs := _toggle("Full screen", Telegram.is_fullscreen(), func(on):
			if on:
				Telegram.request_fullscreen()
			else:
				Telegram.exit_fullscreen())
		left.add_child(fs)
	left.add_child(_toggle("Rotate the game on upright phones", Settings.auto_rotate, func(on):
		Settings.auto_rotate = on
		Settings.save()
		rotation_changed.emit()))
	left.add_child(_toggle("Vibration (haptics)", Settings.haptics, func(on):
		Settings.haptics = on
		Settings.save()))
	if Telegram.at_least("8.0"):
		var home := await Telegram.home_screen_status()
		if home in ["missed", "unknown"] and is_inside_tree():
			left.add_child(UiKit.button("Add to home screen", Telegram.add_to_home_screen, 48, "blue", "home"))

	right.add_child(UiKit.heading("HUD layout", 20))
	right.add_child(UiKit.para("Keep up to three layouts and switch any time. Every element can be moved, resized or hidden.", 15, UiKit.MUTED))
	var slots := HBoxContainer.new()
	for i in range(1, HudLayout.SLOTS + 1):
		var b := UiKit.button("Layout %d" % i, func():
			HudLayout.use_slot(i)
			_render(), 46, "orange" if HudLayout.active == i else "gray")
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		slots.add_child(b)
	right.add_child(slots)
	if in_world:
		right.add_child(UiKit.button("Edit HUD layout", func():
			close()
			edit_layout.emit(), 50, "orange", "layout"))
	var row := HBoxContainer.new()
	row.add_child(UiKit.button("Credits", func(): Popups.open(preload("res://scenes/ui/credits_panel.gd").new()), 46, "gray", "book"))
	if in_world:
		row.add_child(UiKit.button("Leave world", func():
			if await Popups.confirm("Leave the world?", "You will return to character selection.", "Leave", "red"):
				close()
				exit_world.emit(), 46, "red", "exit"))
	right.add_child(row)

func _toggle(text: String, on: bool, cb: Callable) -> CheckButton:
	var c := CheckButton.new()
	c.text = text
	c.button_pressed = on
	c.add_theme_font_size_override("font_size", 18)
	c.custom_minimum_size.y = 44
	c.toggled.connect(func(v):
		Telegram.haptic("selection")
		cb.call(v))
	return c
