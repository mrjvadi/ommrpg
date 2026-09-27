class_name VirtualJoystick
extends Control
## Multi-touch virtual joystick. It reads raw touch events in _input (not the
## emulated mouse) so it keeps working while another finger presses attack.
## With "Emulate Touch From Mouse" it also works with a desktop mouse.

var value := Vector2.ZERO
var radius := 62.0
var _idx := -1
var _knob := Vector2.ZERO
var enabled := true

func _init() -> void:
	custom_minimum_size = Vector2(176, 176)
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
	var r := radius + 20
	var active := _idx != -1
	# stone socket with a bronze ring
	draw_circle(c + Vector2(0, 3), r, Color(0, 0, 0, 0.35))
	draw_circle(c, r, Color(0.05, 0.035, 0.03, 0.55))
	draw_arc(c, r - 2, 0, TAU, 64, Color("0d0908"), 4.0, true)
	draw_arc(c, r - 5, 0, TAU, 64, Color("a57d4c") if not active else Color("ffd35a"), 3.0, true)
	draw_arc(c, r - 9, 0, TAU, 64, Color(0, 0, 0, 0.5), 2.0, true)
	# direction notches
	for i in 4:
		var a := TAU * i / 4.0 - PI / 2
		var p := c + Vector2(cos(a), sin(a)) * (r - 16)
		var t := Vector2(cos(a), sin(a))
		var n := Vector2(-t.y, t.x)
		draw_colored_polygon(PackedVector2Array([p + t * 6, p - t * 3 + n * 6, p - t * 3 - n * 6]), Color(1, 0.9, 0.7, 0.35 if not active else 0.6))
	# metal knob
	var k := c + _knob
	draw_circle(k + Vector2(0, 3), 30, Color(0, 0, 0, 0.45))
	draw_circle(k, 30, Color("0d0908"))
	draw_circle(k, 28, Color("8a7058") if not active else Color("c8a060"))
	draw_circle(k, 23, Color("4e3f33") if not active else Color("7a5a36"))
	draw_circle(k - Vector2(6, 8), 9, Color(1, 1, 1, 0.14))
	draw_arc(k, 26, PI * 1.1, PI * 1.9, 24, Color(1, 1, 1, 0.4), 2.0, true)


class TouchButton extends Control:
	## A round on-screen action button driven by raw touches (multi-touch
	## safe), drawn as a coloured medallion with an icon.
	signal pressed
	signal released

	var text := ""
	var color := Color(1, 1, 1)
	var held := false
	var highlight := false
	var enabled := true
	var _idx := -1
	var _icon: UiIcon
	var _t := 0.0

	func _init(label_text: String, diameter: float, tint: Color, icon_name := "") -> void:
		text = label_text
		color = tint
		custom_minimum_size = Vector2(diameter, diameter)
		mouse_filter = Control.MOUSE_FILTER_IGNORE
		if icon_name != "":
			_icon = UiIcon.new(icon_name, diameter * 0.56, "silver")
			_icon.position = Vector2(diameter * 0.22, diameter * 0.2)
			_icon.size = _icon.custom_minimum_size
			add_child(_icon)

	func _process(delta: float) -> void:
		if highlight:
			_t += delta
			queue_redraw()

	func _input(e: InputEvent) -> void:
		if not enabled or not is_visible_in_tree():
			return
		if e is InputEventScreenTouch:
			var lp: Vector2 = get_global_transform_with_canvas().affine_inverse() * e.position
			if e.pressed and _idx == -1 and lp.distance_to(size / 2) <= size.x / 2 + 8:
				_idx = e.index
				held = true
				pressed.emit()
				Telegram.haptic("light")
				_update_icon()
				queue_redraw()
				get_viewport().set_input_as_handled()
			elif not e.pressed and e.index == _idx:
				_idx = -1
				held = false
				released.emit()
				_update_icon()
				queue_redraw()

	func _update_icon() -> void:
		if _icon:
			_icon.position.y = size.y * 0.2 + (3.0 if held else 0.0)

	func _draw() -> void:
		var c := size / 2
		var r := size.x / 2
		if highlight:
			var pulse := 0.5 + 0.5 * sin(_t * 5.0)
			for i in 5:
				draw_circle(c, r + 10 - i * 2, Color(1, 0.82, 0.3, 0.06 + 0.05 * pulse))
		draw_circle(c + Vector2(0, 4), r, Color(0, 0, 0, 0.45))
		draw_circle(c, r, UiKit.OUTLINE)
		_disc(c, r - 2, Color("e2bd7e") if not highlight else Color("ffe27a"), Color("6a4424"))
		var top := color.lightened(0.3)
		var bottom := color.darkened(0.3)
		if held:
			var tmp := top
			top = bottom
			bottom = tmp
		_disc(c, r - 7, top, bottom)
		draw_arc(c, r - 7, 0, TAU, 48, Color(0, 0, 0, 0.55), 2.0, true)
		if not held:
			var gl := PackedVector2Array()
			for i in 21:
				var a := PI + PI * i / 20.0
				gl.append(c + Vector2(cos(a) * (r - 10), sin(a) * (r - 10) * 0.8 - 2))
			draw_colored_polygon(gl, Color(1, 1, 1, 0.14))
		var ring := UiKit.part("ring")
		if ring:
			draw_texture_rect(ring, Rect2(c - Vector2(r, r) * 1.04, Vector2(r, r) * 2.08), false, Color(1.2, 1.1, 0.9) if highlight else Color.WHITE)
		if _icon == null and text != "":
			var f := UiKit.FONT_BOLD
			var fs := int(r * 0.42)
			var w := f.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, fs).x
			draw_string_outline(f, c + Vector2(-w / 2, fs * 0.35), text, HORIZONTAL_ALIGNMENT_LEFT, -1, fs, 5, UiKit.OUTLINE)
			draw_string(f, c + Vector2(-w / 2, fs * 0.35), text, HORIZONTAL_ALIGNMENT_LEFT, -1, fs, Color.WHITE)

	func _disc(c: Vector2, r: float, a: Color, b: Color) -> void:
		var pts := PackedVector2Array()
		var cols := PackedColorArray()
		for i in 48:
			var ang := TAU * i / 48.0
			var p := c + Vector2(cos(ang), sin(ang)) * r
			pts.append(p)
			cols.append(a.lerp(b, (p.y - (c.y - r)) / (2 * r)))
		draw_polygon(pts, cols)
