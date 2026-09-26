class_name VirtualJoystick
extends Control
## Multi-touch virtual joystick. It reads raw touch events in _input (not the
## emulated mouse) so it keeps working while another finger presses attack.
## With "Emulate Touch From Mouse" it also works with a desktop mouse.

var value := Vector2.ZERO
var radius := 64.0
var _idx := -1
var _knob := Vector2.ZERO
var enabled := true

func _init() -> void:
	custom_minimum_size = Vector2(170, 170)
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func _local(p: Vector2) -> Vector2:
	return get_global_transform_with_canvas().affine_inverse() * p

func _input(e: InputEvent) -> void:
	if not enabled or not is_visible_in_tree():
		return
	if e is InputEventScreenTouch:
		if e.pressed and _idx == -1:
			var lp := _local(e.position)
			if Rect2(Vector2.ZERO, size).grow(24).has_point(lp):
				_idx = e.index
				_update(lp)
				get_viewport().set_input_as_handled()
		elif not e.pressed and e.index == _idx:
			_idx = -1
			value = Vector2.ZERO
			_knob = Vector2.ZERO
			queue_redraw()
	elif e is InputEventScreenDrag and e.index == _idx:
		_update(_local(e.position))
		get_viewport().set_input_as_handled()

func _update(lp: Vector2) -> void:
	var v := lp - size / 2
	if v.length() > radius:
		v = v.normalized() * radius
	_knob = v
	value = v / radius
	if value.length() < 0.18:
		value = Vector2.ZERO
	queue_redraw()

func _draw() -> void:
	var c := size / 2
	draw_circle(c, radius + 14, Color(0, 0, 0, 0.28))
	draw_arc(c, radius + 14, 0, TAU, 48, Color(1, 1, 1, 0.25), 3.0)
	draw_circle(c + _knob, 30, Color(1, 1, 1, 0.35 if _idx == -1 else 0.6))


class TouchButton extends Control:
	## A round on-screen button driven by raw touches (multi-touch safe).
	signal pressed
	signal released

	var text := ""
	var color := Color(1, 1, 1)
	var held := false
	var highlight := false
	var enabled := true
	var _idx := -1
	var _font: Font

	func _init(label_text: String, diameter: float, tint: Color) -> void:
		text = label_text
		color = tint
		custom_minimum_size = Vector2(diameter, diameter)
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _ready() -> void:
		_font = get_theme_default_font()

	func _input(e: InputEvent) -> void:
		if not enabled or not is_visible_in_tree():
			return
		if e is InputEventScreenTouch:
			var lp: Vector2 = get_global_transform_with_canvas().affine_inverse() * e.position
			if e.pressed and _idx == -1 and lp.distance_to(size / 2) <= size.x / 2 + 6:
				_idx = e.index
				held = true
				pressed.emit()
				queue_redraw()
				get_viewport().set_input_as_handled()
			elif not e.pressed and e.index == _idx:
				_idx = -1
				held = false
				released.emit()
				queue_redraw()

	func _draw() -> void:
		var c := size / 2
		var r := size.x / 2
		var base := color
		base.a = 0.55 if not held else 0.85
		draw_circle(c, r, Color(0, 0, 0, 0.35))
		draw_circle(c, r - 4, base)
		draw_arc(c, r - 2, 0, TAU, 48, Color(1, 0.85, 0.3) if highlight else Color(1, 1, 1, 0.4), 3.0)
		if _font:
			var fs := int(r * 0.42)
			var w := _font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, fs).x
			draw_string(_font, c + Vector2(-w / 2, fs * 0.35), text, HORIZONTAL_ALIGNMENT_LEFT, -1, fs, Color.WHITE)
