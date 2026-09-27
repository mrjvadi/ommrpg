extends Node
## Root of the app: switches between the login, character select and world
## screens, and keeps the game in landscape.
##
## The game is designed for a 960x540 landscape canvas (wider phones get
## more width). In Telegram it asks for full screen; once the phone is held
## sideways the orientation is locked. If the phone stays in portrait (for
## example with rotation lock on), the whole game is drawn rotated by 90
## degrees inside a SubViewport, so it is always landscape and input still
## maps correctly.

const LoginScreen := preload("res://scenes/login.gd")
const SelectScreen := preload("res://scenes/character_select.gd")
const WorldScene := preload("res://scenes/world/world.gd")

const BASE := Vector2(960, 540)

## Holds the current screen and the popup layer; moved into the rotated
## viewport when needed.
var stage := Node.new()
var popup_layer := CanvasLayer.new()
var rotated := false
var _current: Node
var _rot_box: SubViewportContainer
var _rot_vp: SubViewport

func _ready() -> void:
	# the theme is built from the Blender GUI kit on the server; wait briefly
	# for it (the built-in look is used if it does not arrive)
	await Pack.wait_loaded(6.0)
	UiKit.install()
	stage.name = "Stage"
	add_child(stage)
	popup_layer.layer = 50
	stage.add_child(popup_layer)
	Popups.set_host(popup_layer)
	get_tree().root.size_changed.connect(_adapt)
	Telegram.fullscreen_changed.connect(func(_on): _adapt())
	Telegram.settings_pressed.connect(open_settings)
	_adapt()
	show_login()

func _touch_device() -> bool:
	return Telegram.is_mobile() or DisplayServer.is_touchscreen_available() or Cfg.force_rotate

func _adapt() -> void:
	var root := get_tree().root
	var win := Vector2(root.size)
	if win.x <= 0 or win.y <= 0:
		return
	var portrait := win.y > win.x
	var want_rot := portrait and _touch_device() and Settings.auto_rotate
	if want_rot != rotated:
		_set_rotated(want_rot)
	if rotated:
		if _rot_vp == null:
			return
		var logical := Vector2(BASE.y * win.y / win.x, BASE.y)
		if logical.x < BASE.x:
			logical = Vector2(BASE.x, BASE.x * win.x / win.y)
		_rot_vp.size = Vector2i(int(win.y), int(win.x))
		_rot_vp.size_2d_override = Vector2i(logical)
		_rot_box.size = Vector2(win.y, win.x)
		_rot_box.position = Vector2(win.x, 0)
		_rot_box.rotation = PI / 2
		Telegram.lock_orientation(false)
	else:
		if root.content_scale_mode != Window.CONTENT_SCALE_MODE_CANVAS_ITEMS:
			root.content_scale_mode = Window.CONTENT_SCALE_MODE_CANVAS_ITEMS
		root.content_scale_size = Vector2i(BASE)
		if not portrait:
			Telegram.lock_orientation(true)
	Telegram.rotated = rotated
	Telegram.insets_changed.emit()

func _set_rotated(on: bool) -> void:
	rotated = on
	var root := get_tree().root
	if on:
		_rot_box = SubViewportContainer.new()
		_rot_box.stretch = false
		_rot_box.mouse_filter = Control.MOUSE_FILTER_STOP
		_rot_vp = SubViewport.new()
		_rot_vp.size_2d_override_stretch = true
		_rot_vp.canvas_item_default_texture_filter = Viewport.DEFAULT_CANVAS_ITEM_TEXTURE_FILTER_NEAREST
		_rot_vp.snap_2d_transforms_to_pixel = true
		_rot_vp.gui_embed_subwindows = true
		_rot_vp.handle_input_locally = true
		_rot_box.add_child(_rot_vp)
		add_child(_rot_box)
		stage.reparent(_rot_vp, false)
		root.content_scale_mode = Window.CONTENT_SCALE_MODE_DISABLED
		Popups.toast("Tip: turn your phone sideways for full screen", UiKit.MUTED)
	else:
		stage.reparent(self, false)
		if _rot_box:
			_rot_box.queue_free()
		_rot_box = null
		_rot_vp = null
		root.content_scale_mode = Window.CONTENT_SCALE_MODE_CANVAS_ITEMS

func _swap(n: Node) -> void:
	Popups.close_all()
	if _current:
		_current.queue_free()
	_current = n
	stage.add_child(n)
	stage.move_child(popup_layer, -1)

func show_login() -> void:
	var s := LoginScreen.new()
	s.logged_in.connect(show_select)
	_swap(s)

func show_select() -> void:
	Telegram.confirm_closing(false)
	var s := SelectScreen.new()
	s.play.connect(show_world)
	_swap(s)

func show_world() -> void:
	Telegram.confirm_closing(true)
	var w := WorldScene.new()
	w.exit_to_menu.connect(func():
		Realtime.close()
		show_select())
	_swap(w)

func open_settings() -> void:
	if Popups.top() is SettingsPanel:
		return
	var p := SettingsPanel.new()
	p.edit_layout.connect(func():
		if _current and _current.has_method("edit_layout"):
			_current.edit_layout())
	p.exit_world.connect(func():
		if _current and _current.has_signal("exit_to_menu"):
			_current.emit_signal("exit_to_menu"))
	p.in_world = _current != null and _current.has_signal("exit_to_menu")
	p.rotation_changed.connect(_adapt)
	Popups.open(p)
