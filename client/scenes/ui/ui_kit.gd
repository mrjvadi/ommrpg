class_name UiKit
extends RefCounted
## Shared look & feel: a dark, slightly pixel-styled theme and small factory
## helpers so every screen is built consistently in code (no fragile .tscn
## layouts to maintain).

const BG := Color("101018")
const PANEL := Color("1b1b2a")
const PANEL_LIGHT := Color("2a2a40")
const ACCENT := Color("f0c040")
const TEXT := Color("ecebf4")
const MUTED := Color("9a98b0")
const DANGER := Color("e05060")
const GOOD := Color("5adc7a")

static var _theme: Theme

static func box(color: Color, radius := 6, border := 0, border_color := Color.TRANSPARENT, pad := 10) -> StyleBoxFlat:
	var s := StyleBoxFlat.new()
	s.bg_color = color
	s.set_corner_radius_all(radius)
	s.set_border_width_all(border)
	s.border_color = border_color
	s.content_margin_left = pad
	s.content_margin_right = pad
	s.content_margin_top = pad * 0.6
	s.content_margin_bottom = pad * 0.6
	return s

static func theme() -> Theme:
	if _theme:
		return _theme
	var t := Theme.new()
	t.default_font = load("res://assets/fonts/Vazirmatn-Regular.ttf")
	t.default_font_size = 20
	t.set_color("font_color", "Label", TEXT)
	t.set_stylebox("normal", "Button", box(PANEL_LIGHT, 8, 2, Color("3c3c5a")))
	t.set_stylebox("hover", "Button", box(Color("35354f"), 8, 2, ACCENT.darkened(0.3)))
	t.set_stylebox("pressed", "Button", box(Color("45456a"), 8, 2, ACCENT))
	t.set_stylebox("disabled", "Button", box(Color("222230"), 8, 2, Color("2a2a3a")))
	t.set_stylebox("focus", "Button", StyleBoxEmpty.new())
	t.set_color("font_color", "Button", TEXT)
	t.set_color("font_disabled_color", "Button", MUTED.darkened(0.3))
	t.set_stylebox("panel", "PanelContainer", box(PANEL, 10, 2, Color("2e2e48"), 14))
	t.set_stylebox("normal", "LineEdit", box(Color("0c0c14"), 6, 2, Color("3c3c5a")))
	t.set_stylebox("focus", "LineEdit", box(Color("0c0c14"), 6, 2, ACCENT))
	t.set_stylebox("background", "ProgressBar", box(Color("0c0c14"), 4, 1, Color("000000"), 0))
	t.set_stylebox("fill", "ProgressBar", box(DANGER, 4, 0, Color.TRANSPARENT, 0))
	t.set_constant("separation", "VBoxContainer", 8)
	t.set_constant("separation", "HBoxContainer", 8)
	var bold: Font = load("res://assets/fonts/Vazirmatn-Bold.ttf")
	t.set_font("font", "TitleLabel", bold)
	_theme = t
	return t

static func label(text: String, size := 20, color := TEXT, bold := false) -> Label:
	var l := Label.new()
	l.text = text
	l.add_theme_font_size_override("font_size", size)
	l.add_theme_color_override("font_color", color)
	if bold:
		l.add_theme_font_override("font", load("res://assets/fonts/Vazirmatn-Bold.ttf"))
	return l

## A label that wraps inside its container's width (for paragraphs).
static func para(text: String, size := 18, color := TEXT, bold := false) -> Label:
	var l := label(text, size, color, bold)
	l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	l.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	l.custom_minimum_size.x = 120
	return l

## Installs the game theme globally (merged into Godot's default theme, so
## it applies everywhere regardless of node hierarchy).
static func install() -> void:
	ThemeDB.get_default_theme().merge_with(theme())

static func button(text: String, cb: Callable, min_h := 48) -> Button:
	var b := Button.new()
	b.text = text
	b.custom_minimum_size = Vector2(0, min_h)
	b.pressed.connect(cb)
	return b

static func panel() -> PanelContainer:
	return PanelContainer.new()

static func bar(fill: Color, h := 14) -> ProgressBar:
	var p := ProgressBar.new()
	p.show_percentage = false
	p.custom_minimum_size = Vector2(0, h)
	p.add_theme_stylebox_override("fill", box(fill, 4, 0, Color.TRANSPARENT, 0))
	return p

static func full_rect(c: Control) -> Control:
	c.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	return c

static func hspacer() -> Control:
	var c := Control.new()
	c.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	return c
