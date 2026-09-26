class_name Medallion
extends Button
## Round icon button with a caption underneath and an optional red "!"
## badge (the HUD menu and action buttons).

var caption: Label
var icon_node: UiIcon
var _badge: Control
var _d := 64.0
var color_kind := "gray"

func _init(icon_name: String, text := "", diameter := 64.0, kind := "gray", palette := "gold") -> void:
	_d = diameter
	color_kind = kind
	focus_mode = Control.FOCUS_NONE
	custom_minimum_size = Vector2(diameter, diameter + (diameter * 0.3 if text != "" else 0.0))
	flat = true
	for st in ["normal", "hover", "pressed", "disabled", "focus", "hover_pressed"]:
		add_theme_stylebox_override(st, StyleBoxEmpty.new())
	var disc := Disc.new(kind)
	disc.position = Vector2.ZERO
	disc.size = Vector2(diameter, diameter)
	disc.name = "Disc"
	add_child(disc)
	icon_node = UiIcon.new(icon_name, diameter * 0.62, palette)
	icon_node.position = Vector2(diameter * 0.19, diameter * 0.17)
	icon_node.size = icon_node.custom_minimum_size
	add_child(icon_node)
	if text != "":
		caption = UiKit.label(text, clampi(int(diameter * 0.27), 12, 22), UiKit.TEXT, true, 5)
		caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		caption.position = Vector2(-diameter * 0.3, diameter * 0.8)
		caption.size = Vector2(diameter * 1.6, diameter * 0.4)
		add_child(caption)
	_badge = UiKit.badge()
	_badge.position = Vector2(diameter * 0.72, -2)
	_badge.visible = false
	add_child(_badge)
	button_down.connect(func(): _press(true))
	button_up.connect(func(): _press(false))

func _press(down: bool) -> void:
	var d: Control = get_node("Disc")
	d.set("pressed", down)
	d.queue_redraw()
	icon_node.position.y = _d * 0.17 + (2.0 if down else 0.0)

func set_badge(on: bool) -> void:
	_badge.visible = on

func set_glow(on: bool) -> void:
	var d: Control = get_node("Disc")
	d.set("glow", on)
	d.queue_redraw()


class Disc extends Control:
	var kind := "gray"
	var pressed := false
	var glow := false

	func _init(k: String) -> void:
		kind = k
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		var c := size / 2
		var r := size.x / 2
		var cols: Array = UiKit.BUTTON_COLORS.get(kind, UiKit.BUTTON_COLORS.gray)
		if kind == "gray":
			cols = [Color("5a4f47"), Color("2a2420"), Color("0d0908")]
		if glow:
			for i in 4:
				draw_circle(c, r + 8 - i * 2, Color(1, 0.8, 0.3, 0.12))
		draw_circle(c + Vector2(0, 3), r, Color(0, 0, 0, 0.45))
		draw_circle(c, r, UiKit.OUTLINE)
		# bronze ring with a vertical gradient
		_disc(c, r - 2, Color("d8b074") if not glow else Color("ffe08a"), Color("6a4424"))
		_disc(c, r - 6, cols[1].darkened(0.35) if not pressed else cols[1].darkened(0.5), cols[0].darkened(0.15) if not pressed else cols[0].darkened(0.35))
		draw_arc(c, r - 6, 0, TAU, 48, Color(0, 0, 0, 0.6), 1.5, true)
		if not pressed:
			draw_arc(c, r - 3.5, PI * 1.1, PI * 1.9, 24, Color(1, 1, 1, 0.45), 1.5, true)

	func _disc(c: Vector2, r: float, top: Color, bottom: Color) -> void:
		var pts := PackedVector2Array()
		var cols := PackedColorArray()
		for i in 40:
			var a := TAU * i / 40.0
			var p := c + Vector2(cos(a), sin(a)) * r
			pts.append(p)
			cols.append(top.lerp(bottom, (p.y - (c.y - r)) / (2 * r)))
		draw_polygon(pts, cols)
