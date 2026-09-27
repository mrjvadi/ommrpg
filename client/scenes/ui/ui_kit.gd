class_name UiKit
extends RefCounted
## The game's look, after classic MMO interfaces (Metin2, Aion): carved
## iron and bronze frames rendered in Blender (Pack GUI kit), recessed stone
## panels, engraved metal buttons, jewelled sockets, Cinzel capitals for
## titles and a warm parchment palette. Every screen is built in code from
## these factories so the style stays consistent. Without the kit the same
## shapes are drawn in code (FancyBox).

# palette
const BG := Color("1a1614")
const PANEL := Color("2c2724")
const PANEL_LIGHT := Color("3d3531")
const ACCENT := Color("f2c55c")
const GOLD := Color("f2c55c")
const TEXT := Color("f4e8cf")
const MUTED := Color("b3a386")
const DIM := Color("7a6e5d")
const DANGER := Color("ff5a4e")
const GOOD := Color("6ee07a")
const INFO := Color("7cc4ff")
const TON := Color("4db8ff")
const ESSENCE := Color("c9a2ff")
const OUTLINE := Color("140d0a")

const RARITY_RIMS := {
	"common": [Color("c6c9cc"), Color("6d7174")],
	"uncommon": [Color("7ee08f"), Color("22793b")],
	"rare": [Color("78b6ff"), Color("1f55b8")],
	"epic": [Color("d59bff"), Color("6c2bb0")],
	"legendary": [Color("ffd06a"), Color("b8610f")],
	"mythic": [Color("ff8a80"), Color("a51c1c")],
	"empty": [Color("5d534b"), Color("2e2723")],
}
const BUTTON_COLORS := {
	"orange": [Color("ffc55a"), Color("e8761c"), Color("3b1c06")],
	"blue": [Color("6cc0ff"), Color("2466d0"), Color("0b2146")],
	"green": [Color("7fe08a"), Color("26944a"), Color("0b3016")],
	"red": [Color("ff7b6b"), Color("c42a22"), Color("3d0907")],
	"purple": [Color("d49bff"), Color("7a36c8"), Color("260c42")],
	"gray": [Color("7d736b"), Color("4a423d"), Color("141010")],
}

## Cinzel capitals (titles, buttons, plaques), Alegreya Sans for text; both
## fall back to Vazirmatn for Persian.
const FONT_TITLE := preload("res://assets/fonts/title.tres")
const FONT_BOLD := preload("res://assets/fonts/body_bold.tres")
const FONT_REGULAR := preload("res://assets/fonts/body.tres")
const STONE := preload("res://assets/ui/stone.png")
const SWITCH_ON := preload("res://assets/ui/switch_on.png")
const SWITCH_OFF := preload("res://assets/ui/switch_off.png")

static var _theme: Theme

# ---------------------------------------------------------------- boxes

## Stone panel with a bronze rim (windows, cards).
static func fancy_panel(pad := 14.0) -> FancyBox:
	var b := FancyBox.new(pad)
	b.radius = 12
	return b

## Recessed dark area inside a panel (stats, lists, inputs).
static func fancy_inset(pad := 10.0) -> FancyBox:
	var b := FancyBox.new(pad)
	b.radius = 8
	b.outline = Color("0b0807")
	b.outline_w = 1.5
	b.rim_top = Color("15110f")
	b.rim_bottom = Color("3a322c")
	b.rim_w = 1.5
	b.fill_top = Color("141110")
	b.fill_bottom = Color("211c1a")
	b.highlight = Color(0, 0, 0, 0)
	b.inset_shadow = 0.45
	b.shadow = Color(1, 1, 1, 0.05)
	b.shadow_offset = Vector2(0, 1.5)
	return b

## Glossy coloured button face.
static func fancy_button(kind: String, state := "normal", pad := 12.0) -> FancyBox:
	var c: Array = BUTTON_COLORS.get(kind, BUTTON_COLORS.gray)
	var b := FancyBox.new(pad)
	b.content_margin_top = pad * 0.45
	b.content_margin_bottom = pad * 0.55 + 3
	b.radius = 10
	b.outline = c[2]
	b.outline_w = 2.0
	b.rim_top = c[0].lightened(0.25)
	b.rim_bottom = c[1].darkened(0.35)
	b.rim_w = 2.0
	b.fill_top = c[0]
	b.fill_bottom = c[1]
	b.inner_line = Color(0, 0, 0, 0.0)
	b.highlight = Color(1, 1, 1, 0.5)
	b.gloss = 0.22
	b.shadow = c[2].darkened(0.3)
	b.shadow_offset = Vector2(0, 4)
	match state:
		"hover":
			b.fill_top = c[0].lightened(0.12)
			b.fill_bottom = c[1].lightened(0.1)
		"pressed":
			b.fill_top = c[1]
			b.fill_bottom = c[0]
			b.gloss = 0.08
			b.shadow_offset = Vector2(0, 1)
			b.content_margin_top += 3
			b.content_margin_bottom -= 3
		"disabled":
			b.fill_top = Color("4a4541")
			b.fill_bottom = Color("302c29")
			b.rim_top = Color("5a534e")
			b.rim_bottom = Color("26221f")
			b.outline = Color("141010")
			b.gloss = 0.05
			b.highlight = Color(1, 1, 1, 0.12)
	return b

## Metallic slot frame coloured by rarity.
static func fancy_slot(rarity: String) -> FancyBox:
	var c: Array = RARITY_RIMS.get(rarity, RARITY_RIMS.empty)
	var b := FancyBox.new(4)
	b.radius = 10
	b.outline = Color("0c0908")
	b.outline_w = 2.0
	b.rim_top = c[0]
	b.rim_bottom = c[1]
	b.rim_w = 4.0
	b.fill_top = Color("4a4440") if rarity != "empty" else Color("2c2724")
	b.fill_bottom = Color("26221f") if rarity != "empty" else Color("1a1614")
	if rarity in ["uncommon", "rare", "epic", "legendary", "mythic"]:
		b.fill_top = c[1].darkened(0.35)
		b.fill_bottom = c[1].darkened(0.75)
	b.highlight = Color(1, 1, 1, 0.55)
	b.inner_line = Color(0, 0, 0, 0.7)
	b.gloss = 0.08
	return b

## Dark capsule behind currency values.
static func fancy_pill() -> FancyBox:
	var b := FancyBox.new(6)
	b.radius = 14
	b.outline = Color("0b0807")
	b.rim_top = Color("4a3f37")
	b.rim_bottom = Color("211b18")
	b.rim_w = 2.0
	b.fill_top = Color("1a1614")
	b.fill_bottom = Color("2a2421")
	b.highlight = Color(1, 1, 1, 0.15)
	b.inset_shadow = 0.4
	return b

static func fancy_bar_bg() -> FancyBox:
	var b := FancyBox.new(0)
	b.radius = 7
	b.outline = Color("0b0807")
	b.rim_top = Color("16110f")
	b.rim_bottom = Color("4a3f37")
	b.rim_w = 1.5
	b.fill_top = Color("0e0b0a")
	b.fill_bottom = Color("231d1a")
	b.highlight = Color(0, 0, 0, 0)
	b.inset_shadow = 0.5
	b.shadow = Color(1, 1, 1, 0.06)
	b.shadow_offset = Vector2(0, 1.5)
	return b

static func fancy_bar_fill(color: Color) -> FancyBox:
	var b := FancyBox.new(0)
	b.radius = 6
	b.outline = Color(0, 0, 0, 0)
	b.outline_w = 2.0
	b.rim_w = 0.0
	b.fill_top = color.lightened(0.3)
	b.fill_bottom = color.darkened(0.25)
	b.gloss = 0.3
	b.inner_line = Color(0, 0, 0, 0)
	b.highlight = Color(0, 0, 0, 0)
	b.shadow = Color(0, 0, 0, 0)
	return b

# ---------------------------------------------------------------- kit boxes
# The GUI kit rendered with Blender (Pack) when the server has it, the
# code-drawn FancyBox look otherwise.

static func _kit(part: String, pad: Array, scale := 0.0) -> StyleBox:
	return Pack.nine(part, pad, scale)

## Padding that clears a part's painted border (its "content" anchor).
static func _pad(part: String, extra: float, scale := 0.5) -> Array:
	var c := float(Pack.anchors(part).get("content", 8)) * scale + extra
	return [c, c, c, c]

## Big windows: carved iron frame with an engraved band and jewelled
## bronze corner brackets.
static func window_box(pad := 16.0) -> StyleBox:
	var k := _kit("window", _pad("window", pad), 0.5)
	return k if k else fancy_panel(pad)

## Cards and raised panels (item cards, character cards).
static func card_box(pad := 12.0) -> StyleBox:
	var k := _kit("card", _pad("card", pad))
	return k if k else fancy_panel(pad)

## Panels inside windows.
static func panel_box(pad := 14.0) -> StyleBox:
	var k := _kit("panel", _pad("panel", pad))
	return k if k else fancy_panel(pad)

## Recessed areas (stats, lists).
static func inset_box(pad := 10.0) -> StyleBox:
	var k := _kit("panel", _pad("panel", pad))
	return k if k else fancy_inset(pad)

## Text fields.
static func input_box(pad := 8.0) -> StyleBox:
	var k := _kit("input", _pad("input", pad))
	return k if k else fancy_inset(pad)

## Tooltips and small floating notes.
static func tooltip_box(pad := 8.0) -> StyleBox:
	var k := _kit("tooltip", _pad("tooltip", pad))
	return k if k else fancy_panel(pad)

## Translucent chat / log panel.
static func chat_box(pad := 6.0) -> StyleBox:
	var k := _kit("chat", _pad("chat", pad))
	return k if k else box(Color(0, 0, 0, 0.45), 6, 0, Color.TRANSPARENT, int(pad))

static func button_box(kind: String, state := "normal", pad := 12.0) -> StyleBox:
	var k := _kit("button_%s_%s" % [kind, state], [pad + 8, 6 if state != "pressed" else 8, pad + 8, 7 if state != "pressed" else 5])
	return k if k else fancy_button(kind, state, pad)

static func tab_box(active: bool) -> StyleBox:
	var k := _kit("tab_on" if active else "tab", [18, 7, 18, 6])
	return k if k else fancy_button("orange" if active else "gray")

static func slot_box(rarity: String) -> StyleBox:
	var k := _kit("slot_" + rarity, [4, 4, 4, 4])
	return k if k else fancy_slot(rarity)

static func pill_box() -> StyleBox:
	var k := _kit("pill", [36, 4, 12, 4])
	return k if k else fancy_pill()

static func bar_bg() -> StyleBox:
	var k := _kit("bar_frame", [0, 0, 0, 0])
	return k if k else fancy_bar_bg()

## Glass tube of the nearest kit colour.
static func bar_fill(color: Color) -> StyleBox:
	var name := "red"
	if color.s < 0.25:
		name = "gold"
	elif color.h > 0.52 and color.h < 0.72:
		name = "blue"
	elif color.h >= 0.72 and color.h < 0.92:
		name = "purple"
	elif color.h > 0.2 and color.h <= 0.52:
		name = "green"
	elif color.h > 0.06 and color.h <= 0.2:
		name = "gold"
	var k := _kit("bar_" + name, [0, 0, 0, 0])
	if k:
		# the tube sits inside the bar frame
		(k as NineBox).inset = 4.0
		return k
	return fancy_bar_fill(color)

## Title / level banner: the kit's metal plaque, or the red ribbon.
static func banner(min_size := Vector2(300, 48)) -> Control:
	var plaque := Pack.nine("plaque", [40, 8, 40, 8], 0.5)
	if plaque == null:
		return Ribbon.new(min_size)
	var p := Panel.new()
	p.add_theme_stylebox_override("panel", plaque)
	p.custom_minimum_size = min_size
	p.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return p

## A kit part as a texture (rings, separators, close button) or null.
static func part(name: String) -> Texture2D:
	return Pack.part_texture(name)

## Brightens / tints any box (kit or code-drawn) for selected or hover states.
static func tint(b: StyleBox, c: Color) -> StyleBox:
	if b is NineBox:
		b.modulate = c
	elif b is FancyBox:
		b.rim_top = b.rim_top * c
		b.rim_bottom = b.rim_bottom * c
	return b

## Backwards-compatible flat box (used for small overlays).
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
	s.anti_aliasing = true
	return s

# ---------------------------------------------------------------- theme

static func theme() -> Theme:
	if _theme:
		return _theme
	var t := Theme.new()
	t.default_font = FONT_BOLD
	t.default_font_size = 18
	for type in ["Label", "Button", "CheckButton", "OptionButton", "LinkButton", "CheckBox"]:
		t.set_color("font_outline_color", type, OUTLINE)
		t.set_constant("outline_size", type, 4)
	for type in ["Button", "OptionButton"]:
		t.set_font("font", type, FONT_TITLE)
	t.set_color("font_color", "Label", TEXT)
	t.set_color("font_shadow_color", "Label", Color(0, 0, 0, 0.45))
	t.set_constant("shadow_offset_y", "Label", 2)
	t.set_constant("shadow_offset_x", "Label", 0)
	for type in ["Button", "OptionButton"]:
		t.set_stylebox("normal", type, button_box("gray"))
		t.set_stylebox("hover", type, button_box("gray", "hover"))
		t.set_stylebox("pressed", type, button_box("gray", "pressed"))
		t.set_stylebox("hover_pressed", type, button_box("gray", "pressed"))
		t.set_stylebox("disabled", type, button_box("gray", "disabled"))
		t.set_stylebox("focus", type, StyleBoxEmpty.new())
		t.set_color("font_color", type, TEXT)
		t.set_color("font_hover_color", type, Color.WHITE)
		t.set_color("font_pressed_color", type, TEXT)
		t.set_color("font_focus_color", type, TEXT)
		t.set_color("font_disabled_color", type, DIM)
	t.set_stylebox("panel", "PanelContainer", panel_box())
	t.set_stylebox("panel", "Panel", panel_box())
	var edit := input_box(8)
	t.set_stylebox("normal", "LineEdit", edit)
	var edit_focus: StyleBox = tint(edit.duplicate() if edit is NineBox else (edit as FancyBox).copy(), Color(1.35, 1.2, 0.85))
	t.set_stylebox("focus", "LineEdit", edit_focus)
	t.set_stylebox("read_only", "LineEdit", edit)
	t.set_font("font", "LineEdit", FONT_REGULAR)
	t.set_color("font_color", "LineEdit", TEXT)
	t.set_color("font_placeholder_color", "LineEdit", DIM)
	t.set_color("font_uneditable_color", "LineEdit", MUTED)
	t.set_color("caret_color", "LineEdit", GOLD)
	t.set_color("selection_color", "LineEdit", Color(1, 0.8, 0.3, 0.35))
	t.set_stylebox("background", "ProgressBar", bar_bg())
	t.set_stylebox("fill", "ProgressBar", bar_fill(DANGER))
	t.set_color("font_color", "CheckButton", TEXT)
	t.set_color("font_hover_color", "CheckButton", Color.WHITE)
	t.set_color("font_pressed_color", "CheckButton", TEXT)
	t.set_color("font_hover_pressed_color", "CheckButton", Color.WHITE)
	for ic in ["checked", "checked_mirrored"]:
		t.set_icon(ic, "CheckButton", SWITCH_ON)
	for ic in ["unchecked", "unchecked_mirrored"]:
		t.set_icon(ic, "CheckButton", SWITCH_OFF)
	for ic in ["checked_disabled", "checked_disabled_mirrored"]:
		t.set_icon(ic, "CheckButton", SWITCH_ON)
	for ic in ["unchecked_disabled", "unchecked_disabled_mirrored"]:
		t.set_icon(ic, "CheckButton", SWITCH_OFF)
	for st in ["normal", "hover", "pressed", "hover_pressed", "focus", "disabled"]:
		t.set_stylebox(st, "CheckButton", StyleBoxEmpty.new())
	var popup := tooltip_box(4)
	t.set_stylebox("panel", "PopupMenu", popup)
	t.set_stylebox("hover", "PopupMenu", fancy_button("orange", "normal", 6))
	t.set_color("font_color", "PopupMenu", TEXT)
	t.set_color("font_hover_color", "PopupMenu", Color.WHITE)
	t.set_font("font", "PopupMenu", FONT_TITLE)
	t.set_font_size("font_size", "PopupMenu", 18)
	t.set_color("font_outline_color", "PopupMenu", OUTLINE)
	t.set_constant("outline_size", "PopupMenu", 4)
	t.set_constant("v_separation", "PopupMenu", 10)
	var grab := FancyBox.new(0)
	grab.radius = 4
	grab.rim_w = 0
	grab.fill_top = Color("8a7766")
	grab.fill_bottom = Color("5a4b3f")
	grab.shadow = Color(0, 0, 0, 0)
	grab.inner_line = Color(0, 0, 0, 0)
	grab.highlight = Color(0, 0, 0, 0)
	var track := StyleBoxFlat.new()
	track.bg_color = Color(0, 0, 0, 0.25)
	track.set_corner_radius_all(4)
	track.content_margin_left = 6
	track.content_margin_right = 6
	for sb in ["VScrollBar", "HScrollBar"]:
		t.set_stylebox("scroll", sb, track)
		t.set_stylebox("grabber", sb, grab)
		t.set_stylebox("grabber_highlight", sb, grab)
		t.set_stylebox("grabber_pressed", sb, grab)
	t.set_constant("separation", "VBoxContainer", 8)
	t.set_constant("separation", "HBoxContainer", 8)
	t.set_constant("h_separation", "GridContainer", 8)
	t.set_constant("v_separation", "GridContainer", 8)
	t.set_font("font", "TitleLabel", FONT_TITLE)
	var tip := tooltip_box(6)
	t.set_stylebox("panel", "TooltipPanel", tip)
	t.set_color("font_color", "TooltipLabel", TEXT)
	_theme = t
	return t

## Installs the game theme globally (merged into Godot's default theme, so
## it applies everywhere regardless of node hierarchy).
static func install() -> void:
	ThemeDB.get_default_theme().merge_with(theme())

# ---------------------------------------------------------------- text

static func label(text: String, size := 18, color := TEXT, bold := true, outline := -1) -> Label:
	var l := Label.new()
	l.text = text
	l.add_theme_font_size_override("font_size", size)
	l.add_theme_color_override("font_color", color)
	l.add_theme_font_override("font", FONT_BOLD if bold else FONT_REGULAR)
	var o := outline if outline >= 0 else clampi(int(size / 5.0), 2, 6)
	l.add_theme_constant_override("outline_size", o)
	l.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return l

## A label that wraps inside its container's width (for paragraphs).
static func para(text: String, size := 16, color := TEXT, bold := false) -> Label:
	var l := label(text, size, color, bold, 0 if not bold else -1)
	l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	l.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	l.custom_minimum_size.x = 120
	if not bold:
		l.add_theme_constant_override("shadow_offset_y", 1)
	return l

## Engraved-gold capitals (Cinzel) for window titles and plaques.
static func title(text: String, size := 30) -> Label:
	var l := label(text, size, GOLD, true, maxi(3, int(size / 6.0)))
	l.add_theme_font_override("font", FONT_TITLE)
	l.add_theme_color_override("font_shadow_color", Color(0, 0, 0, 0.7))
	l.add_theme_constant_override("shadow_offset_y", 2)
	return l

## Small capitals for section headings inside windows.
static func heading(text: String, size := 17, color := GOLD) -> Label:
	var l := label(text, size, color, true, 3)
	l.add_theme_font_override("font", FONT_TITLE)
	return l

# ---------------------------------------------------------------- controls

## Chunky glossy button. kind: orange (primary), blue, green, red, purple, gray.
static func button(text: String, cb: Callable, min_h := 48, kind := "gray", icon := "") -> Button:
	var b := Button.new()
	b.text = text
	b.custom_minimum_size = Vector2(0, min_h)
	b.add_theme_font_size_override("font_size", clampi(int(min_h * 0.36), 13, 24))
	style_button(b, kind)
	if icon != "":
		var ic := UiIcon.new(icon, min_h * 0.5, "silver" if kind != "gray" else "gold")
		b.add_child(ic)
		b.set_meta("icon_node", ic)
		b.resized.connect(func(): ic.position = Vector2(10, (b.size.y - ic.custom_minimum_size.y) / 2 - 1))
		b.add_theme_constant_override("h_separation", 0)
		b.alignment = HORIZONTAL_ALIGNMENT_CENTER
		b.text = "      " + text if text != "" else ""
	b.pressed.connect(func(): Telegram.haptic("light"))
	b.pressed.connect(cb)
	return b

static func style_button(b: Button, kind: String) -> void:
	for st in ["normal", "hover", "pressed", "disabled"]:
		b.add_theme_stylebox_override(st, button_box(kind, st))
	b.add_theme_stylebox_override("hover_pressed", button_box(kind, "pressed"))

static func panel(pad := 14.0) -> PanelContainer:
	var p := PanelContainer.new()
	if pad != 14.0:
		p.add_theme_stylebox_override("panel", panel_box(pad))
	return p

## A window-style panel with the carved frame.
static func window(pad := 16.0) -> PanelContainer:
	var p := PanelContainer.new()
	p.add_theme_stylebox_override("panel", window_box(pad))
	return p

## Ornamental divider line.
static func separator(width := 300.0) -> Control:
	var tex := part("separator")
	if tex == null:
		var c := ColorRect.new()
		c.color = Color(0.6, 0.45, 0.25, 0.5)
		c.custom_minimum_size = Vector2(width, 2)
		return c
	var tr := TextureRect.new()
	tr.texture = tex
	tr.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	tr.stretch_mode = TextureRect.STRETCH_SCALE
	tr.custom_minimum_size = Vector2(width, 14)
	tr.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	tr.texture_filter = CanvasItem.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS
	tr.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return tr

static func inset(pad := 10.0) -> PanelContainer:
	var p := PanelContainer.new()
	p.add_theme_stylebox_override("panel", inset_box(pad))
	return p

static func bar(fill: Color, h := 14) -> ProgressBar:
	var p := ProgressBar.new()
	p.show_percentage = false
	p.custom_minimum_size = Vector2(0, h)
	p.add_theme_stylebox_override("fill", bar_fill(fill))
	return p

## A bar with a centred "value / max" caption.
static func value_bar(fill: Color, h := 22) -> ProgressBar:
	var p := bar(fill, h)
	var l := label("", clampi(int(h * 0.62), 11, 20), TEXT, true, 4)
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	full_rect(l)
	l.offset_top = -2
	p.add_child(l)
	p.set_meta("caption", l)
	return p

static func set_bar(p: ProgressBar, value: float, max_value: float, caption := "") -> void:
	p.max_value = maxf(1.0, max_value)
	p.value = value
	if p.has_meta("caption"):
		(p.get_meta("caption") as Label).text = caption if caption != "" else "%d / %d" % [int(value), int(max_value)]

## Red "!" notification dot.
static func badge(text := "!") -> Control:
	var b := Badge.new()
	b.text = text
	return b

## Round red close button.
static func close_button(cb: Callable) -> Button:
	var b := Button.new()
	b.custom_minimum_size = Vector2(46, 46)
	b.add_theme_stylebox_override("focus", StyleBoxEmpty.new())
	var tex := part("close")
	if tex:
		for st in ["normal", "hover", "pressed"]:
			b.add_theme_stylebox_override(st, StyleBoxEmpty.new())
		var tr := TextureRect.new()
		tr.texture = tex
		tr.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
		tr.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
		tr.texture_filter = CanvasItem.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS
		tr.mouse_filter = Control.MOUSE_FILTER_IGNORE
		full_rect(tr)
		b.add_child(tr)
		b.button_down.connect(func(): tr.modulate = Color(0.8, 0.8, 0.8))
		b.button_up.connect(func(): tr.modulate = Color.WHITE)
	else:
		for st in ["normal", "hover", "pressed"]:
			var s := fancy_button("red", st, 0)
			s.radius = 23
			b.add_theme_stylebox_override(st, s)
		var x := CrossMark.new()
		full_rect(x)
		b.add_child(x)
	b.pressed.connect(func(): Telegram.haptic("light"))
	b.pressed.connect(cb)
	return b

static func icon(name: String, size := 32.0, palette := "gold") -> UiIcon:
	return UiIcon.new(name, size, palette)

## Icon + value in a dark capsule, with an optional orange "+" button.
static func pill(icon_name: String, palette: String, plus_cb := Callable()) -> Pill:
	return Pill.new(icon_name, palette, plus_cb)

## Tiled stone wall with a vignette (screen backgrounds).
static func backdrop() -> Control:
	var root := Control.new()
	full_rect(root)
	root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var t := TextureRect.new()
	t.texture = STONE
	t.stretch_mode = TextureRect.STRETCH_TILE
	t.texture_filter = CanvasItem.TEXTURE_FILTER_LINEAR
	t.modulate = Color(0.78, 0.74, 0.72)
	root.add_child(full_rect(t))
	var v := Vignette.new()
	root.add_child(full_rect(v))
	return root

## Stat line "◆ HP   5 937" (label muted, value bright).
static func stat_row(name: String, value: String, value_color := TEXT, size := 17) -> HBoxContainer:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 6)
	var d := Diamond.new()
	row.add_child(d)
	var n := label(name, size, MUTED, true, 4)
	n.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(n)
	var v := label(value, size, value_color, true, 4)
	v.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	row.add_child(v)
	return row

## Wraps a control so it gets a tab-like header strip (used by windows).
static func tabs(names: Array, current: String, on_pick: Callable, min_h := 44) -> HBoxContainer:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 6)
	for pair in names:
		var id: String = pair[0]
		var b := button(pair[1], func():
			Telegram.haptic("selection")
			on_pick.call(id), min_h, "orange" if id == current else "gray", pair[2] if pair.size() > 2 else "")
		if Pack.has_part("tab"):
			for st in ["normal", "hover", "pressed", "hover_pressed", "disabled"]:
				b.add_theme_stylebox_override(st, tab_box(id == current))
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.set_meta("tab", id)
		row.add_child(b)
	return row

static func full_rect(c: Control) -> Control:
	c.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	return c

static func hspacer() -> Control:
	var c := Control.new()
	c.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	c.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return c

static func vspacer() -> Control:
	var c := Control.new()
	c.size_flags_vertical = Control.SIZE_EXPAND_FILL
	c.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return c

static func margin(c: Control, m: float) -> MarginContainer:
	var mc := MarginContainer.new()
	for side in ["left", "right", "top", "bottom"]:
		mc.add_theme_constant_override("margin_" + side, int(m))
	mc.add_child(c)
	return mc

## Number with thin-space thousands separators (59 437).
static func num(v) -> String:
	var s := str(int(v))
	var neg := s.begins_with("-")
	if neg:
		s = s.substr(1)
	var out := ""
	while s.length() > 3:
		out = " " + s.substr(s.length() - 3) + out
		s = s.substr(0, s.length() - 3)
	return ("-" if neg else "") + s + out

## Compact number (12.5K, 3.2M) for tight spaces.
static func short(v) -> String:
	var f := float(v)
	if absf(f) >= 1_000_000:
		return ("%.1fM" % (f / 1_000_000.0)).replace(".0M", "M")
	if absf(f) >= 10_000:
		return ("%.1fK" % (f / 1000.0)).replace(".0K", "K")
	return num(v)


class Badge extends Control:
	## Small red circle with a white "!" (or a count).
	var text := "!"

	func _init() -> void:
		custom_minimum_size = Vector2(24, 24)
		size = custom_minimum_size
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		var c := size / 2
		var r := minf(size.x, size.y) / 2
		draw_circle(c + Vector2(0, 1.5), r, Color(0, 0, 0, 0.45))
		draw_circle(c, r, Color("fff4e2"))
		draw_circle(c, r - 2.5, Color("e3261c"))
		draw_circle(c - Vector2(0, r * 0.3), r * 0.45, Color(1, 1, 1, 0.18))
		var f := UiKit.FONT_BOLD
		var fs := int(r * 1.3)
		var w := f.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, fs).x
		draw_string(f, c + Vector2(-w / 2, fs * 0.36), text, HORIZONTAL_ALIGNMENT_LEFT, -1, fs, Color.WHITE)


class CrossMark extends Control:
	func _init() -> void:
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		var c := size / 2 - Vector2(0, 1.5)
		var r := minf(size.x, size.y) * 0.2
		for w in [7.0, 4.0]:
			var col := UiKit.OUTLINE if w > 5 else Color.WHITE
			draw_line(c + Vector2(-r, -r), c + Vector2(r, r), col, w, true)
			draw_line(c + Vector2(r, -r), c + Vector2(-r, r), col, w, true)


class Diamond extends Control:
	## The ◆ bullet in front of stat names.
	func _init() -> void:
		custom_minimum_size = Vector2(12, 12)
		size_flags_vertical = Control.SIZE_SHRINK_CENTER
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		var c := size / 2
		var pts := PackedVector2Array([c + Vector2(0, -6), c + Vector2(6, 0), c + Vector2(0, 6), c + Vector2(-6, 0)])
		draw_colored_polygon(pts, UiKit.OUTLINE)
		var inner := PackedVector2Array([c + Vector2(0, -3.8), c + Vector2(3.8, 0), c + Vector2(0, 3.8), c + Vector2(-3.8, 0)])
		draw_colored_polygon(inner, Color("a89680"))


class Vignette extends Control:
	## Darkens the screen edges (drawn with a few gradient quads).
	func _init() -> void:
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		var s := size
		var dark := Color(0, 0, 0, 0.72)
		var clear := Color(0, 0, 0, 0)
		var e := minf(s.x, s.y) * 0.45
		draw_polygon(PackedVector2Array([Vector2(0, 0), Vector2(s.x, 0), Vector2(s.x, e), Vector2(0, e)]), PackedColorArray([dark, dark, clear, clear]))
		draw_polygon(PackedVector2Array([Vector2(0, s.y - e), Vector2(s.x, s.y - e), Vector2(s.x, s.y), Vector2(0, s.y)]), PackedColorArray([clear, clear, dark, dark]))
		draw_polygon(PackedVector2Array([Vector2(0, 0), Vector2(e, 0), Vector2(e, s.y), Vector2(0, s.y)]), PackedColorArray([dark, clear, clear, dark]))
		draw_polygon(PackedVector2Array([Vector2(s.x - e, 0), Vector2(s.x, 0), Vector2(s.x, s.y), Vector2(s.x - e, s.y)]), PackedColorArray([clear, dark, dark, clear]))

	func _notification(what: int) -> void:
		if what == NOTIFICATION_RESIZED:
			queue_redraw()


class Pill extends PanelContainer:
	## Currency display: icon, value and an optional orange "+" button.
	var value_label: Label
	var _icon: UiIcon

	func _init(icon_name: String, palette: String, plus_cb := Callable()) -> void:
		add_theme_stylebox_override("panel", UiKit.pill_box())
		var row := HBoxContainer.new()
		row.add_theme_constant_override("separation", 4)
		add_child(row)
		_icon = UiIcon.new(icon_name, 30, palette)
		if Pack.has_part("pill"):
			# the icon sits in the pill's round socket, over the left border
			var holder := Control.new()
			holder.mouse_filter = Control.MOUSE_FILTER_IGNORE
			row.add_child(holder)
			holder.add_child(_icon)
			_icon.position = Vector2(-36 + 15 - 15, -15)
			holder.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		else:
			row.add_child(_icon)
		value_label = UiKit.label("0", 17, UiKit.TEXT, true, 3)
		value_label.custom_minimum_size.x = 46
		value_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		row.add_child(value_label)
		if plus_cb.is_valid():
			var plus := Button.new()
			plus.custom_minimum_size = Vector2(26, 26)
			for st in ["normal", "hover", "pressed"]:
				if Pack.has_part("button_orange_normal"):
					plus.add_theme_stylebox_override(st, UiKit.button_box("orange", st, 0))
					continue
				var s := UiKit.fancy_button("orange", st, 0)
				s.radius = 7
				s.shadow_offset = Vector2(0, 2)
				plus.add_theme_stylebox_override(st, s)
			plus.add_theme_stylebox_override("focus", StyleBoxEmpty.new())
			var mark := PlusMark.new()
			UiKit.full_rect(mark)
			plus.add_child(mark)
			plus.pressed.connect(plus_cb)
			row.add_child(plus)

	func set_value(text: String) -> void:
		value_label.text = text


class PlusMark extends Control:
	func _init() -> void:
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		var c := size / 2 - Vector2(0, 1.5)
		var r := minf(size.x, size.y) * 0.22
		for w in [5.0, 2.5]:
			var col := UiKit.OUTLINE if w > 4 else Color("fff2c8")
			draw_line(c + Vector2(-r, 0), c + Vector2(r, 0), col, w, true)
			draw_line(c + Vector2(0, -r), c + Vector2(0, r), col, w, true)
