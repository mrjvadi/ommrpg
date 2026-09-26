extends Node
## Window and popup manager. Every window (Modal) and dialog goes through
## here, so they stack correctly, the Telegram Back button / Escape close
## the top one, and toasts stay visible above them.
##
##   Popups.open(TradePanel.new())
##   if await Popups.confirm("Salvage?", "It turns into essence.", "Salvage", "red"): ...
##   if await Popups.money_confirm("Buy for 4.5 TON?", "..."): ...

signal changed

## The CanvasLayer windows are added to (set by main.gd; it lives inside the
## rotated stage when the game is drawn sideways).
var host: CanvasLayer
## True while the in-world HUD shows messages itself.
var hud_log_active := false
var stack: Array = []
var _toasts: VBoxContainer

func _ready() -> void:
	Telegram.back_pressed.connect(close_top)
	Game.notify.connect(_on_notify)

func set_host(layer: CanvasLayer) -> void:
	host = layer
	_toasts = VBoxContainer.new()
	_toasts.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_toasts.alignment = BoxContainer.ALIGNMENT_BEGIN
	_toasts.z_index = 100
	layer.add_child(_toasts)
	_place_toasts()
	layer.get_viewport().size_changed.connect(_place_toasts)

func _place_toasts() -> void:
	if not is_instance_valid(_toasts):
		return
	var vs := _toasts.get_viewport_rect().size
	var ins := Telegram.insets_for(_toasts.get_viewport())
	_toasts.position = Vector2(vs.x * 0.25, ins.x + 12)
	_toasts.size = Vector2(vs.x * 0.5, 10)

func open(m: Control) -> Control:
	if host == null:
		push_error("Popups.open before a host was set")
		return m
	host.add_child(m)
	if is_instance_valid(_toasts):
		host.move_child(_toasts, -1)
	stack.append(m)
	m.tree_exited.connect(func():
		stack.erase(m)
		_update())
	_update()
	return m

func has_open() -> bool:
	return not stack.is_empty()

func top() -> Control:
	return stack.back() if not stack.is_empty() else null

func close_top() -> bool:
	var m := top()
	if m == null:
		return false
	if m.has_method("close"):
		m.close()
	else:
		m.queue_free()
	return true

func close_all() -> void:
	for m in stack.duplicate():
		if is_instance_valid(m):
			m.queue_free()
	stack.clear()
	_update()

func _update() -> void:
	Telegram.set_back_button(not stack.is_empty())
	changed.emit()

func _unhandled_input(e: InputEvent) -> void:
	if e is InputEventKey and e.pressed and not e.echo and e.physical_keycode == KEY_ESCAPE:
		if close_top():
			get_viewport().set_input_as_handled()

# ---------------------------------------------------------------- dialogs

## In-game confirmation dialog. Returns true for the confirm button.
func confirm(title: String, message: String, ok_text := "OK", kind := "orange", cancel_text := "Cancel") -> bool:
	var d := Dialog.new(title, message)
	d.add_button(cancel_text, "gray", false)
	d.add_button(ok_text, kind, true)
	open(d)
	var res: bool = await d.answered
	return res

func alert(title: String, message: String) -> void:
	var d := Dialog.new(title, message)
	d.add_button("OK", "orange", true)
	open(d)
	await d.answered

## Real-money confirmation: Telegram's native popup when available (it
## cannot be imitated by page content), the in-game dialog otherwise.
func money_confirm(title: String, message: String, ok_text: String) -> bool:
	var id := await Telegram.popup(title, message, [{"id": "no", "type": "cancel"}, {"id": "yes", "type": "destructive", "text": ok_text}])
	if id == "unsupported":
		return await confirm(title, message, ok_text, "blue")
	return id == "yes"

# ---------------------------------------------------------------- toasts

func _on_notify(text: String, color: Color) -> void:
	if hud_log_active and stack.is_empty():
		return # the HUD shows it in its own (movable) message log
	toast(text, color)

func toast(text: String, color := Color.WHITE) -> void:
	if not is_instance_valid(_toasts):
		return
	var p := PanelContainer.new()
	var sb := UiKit.box(Color(0.06, 0.04, 0.03, 0.82), 12, 2, Color(color, 0.6), 14)
	p.add_theme_stylebox_override("panel", sb)
	p.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var l := UiKit.label(text, 18, color, true, 5)
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	p.add_child(l)
	_toasts.add_child(p)
	while _toasts.get_child_count() > 4:
		_toasts.get_child(0).free()
	p.modulate.a = 0.0
	var tw := p.create_tween()
	tw.tween_property(p, "modulate:a", 1.0, 0.15)
	tw.tween_interval(2.8)
	tw.tween_property(p, "modulate:a", 0.0, 0.5)
	tw.tween_callback(p.queue_free)


class Dialog extends Modal:
	## Small window with a message and a row of buttons.
	signal answered(ok: bool)
	var _row: HBoxContainer
	var _answered := false

	func _init(title: String, message: String) -> void:
		super(title)
		max_size = Vector2(560, 330)
		scrollable = false
		fit_content = true
		var msg := UiKit.para(message, 19, UiKit.TEXT)
		msg.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		msg.custom_minimum_size.y = 90
		msg.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		body.add_child(msg)
		_row = HBoxContainer.new()
		_row.alignment = BoxContainer.ALIGNMENT_CENTER
		_row.add_theme_constant_override("separation", 16)
		body.add_child(_row)
		closed.connect(func(): _answer(false))

	func add_button(text: String, kind: String, value: bool) -> void:
		var b := UiKit.button(text, func():
			_answer(value)
			close(), 54, kind)
		b.custom_minimum_size.x = 170
		_row.add_child(b)

	func _answer(v: bool) -> void:
		if _answered:
			return
		_answered = true
		answered.emit(v)
