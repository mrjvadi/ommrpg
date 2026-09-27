class_name StatusPlate
extends KitPart
## The HUD's character plate (after Metin2 / Aion): portrait in a jewelled
## orb, the name in an engraved channel, health and experience as glass
## tubes in the plate's channels and the level in a socket under the orb.
## Every widget is placed on the anchors the Blender kit exports.

signal portrait_pressed

const FB_SIZE := Vector2(820, 256)
const FB_ANCHORS := {
	"name": [260, 60, 360, 32], "hp": [260, 104, 360, 28], "xp": [260, 148, 360, 24],
	"orb": [130, 128, 96], "badge": [206, 208, 22],
}

var name_label: Label
var class_label: Label
var level_label: Label
var hp_bar: ProgressBar
var xp_bar: ProgressBar
var portrait: Portrait

func _init(k := 0.5) -> void:
	super._init("status", k, FB_SIZE, FB_ANCHORS)
	var orb := circle("orb")
	portrait = Portrait.new(orb.z * 2)
	portrait.bare = available()
	portrait.position = Vector2(orb.x, orb.y) - Vector2(orb.z, orb.z)
	portrait.size = Vector2(orb.z, orb.z) * 2
	var tap := Button.new()
	tap.flat = true
	tap.focus_mode = Control.FOCUS_NONE
	for st in ["normal", "hover", "pressed", "focus", "hover_pressed"]:
		tap.add_theme_stylebox_override(st, StyleBoxEmpty.new())
	tap.position = portrait.position
	tap.size = portrait.size
	tap.pressed.connect(func(): portrait_pressed.emit())
	add_child(portrait)
	add_child(tap)

	var nr := rect("name")
	name_label = UiKit.label("", int(nr.size.y * 0.72), UiKit.TEXT, true, 3)
	name_label.add_theme_font_override("font", UiKit.FONT_TITLE)
	name_label.position = nr.position + Vector2(8, -1)
	name_label.size = Vector2(nr.size.x * 0.62, nr.size.y)
	name_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	name_label.clip_text = true
	add_child(name_label)
	class_label = UiKit.label("", int(nr.size.y * 0.5), UiKit.MUTED, true, 2)
	class_label.position = nr.position + Vector2(nr.size.x * 0.6, 0)
	class_label.size = Vector2(nr.size.x * 0.4 - 8, nr.size.y)
	class_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	class_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	add_child(class_label)

	hp_bar = _tube("hp", Color("e0413a"))
	xp_bar = _tube("xp", Color("e8a93a"))

	var b := circle("badge")
	level_label = UiKit.label("1", int(b.z * 1.05), UiKit.GOLD, true, 3)
	level_label.add_theme_font_override("font", UiKit.FONT_TITLE)
	level_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	level_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	level_label.position = Vector2(b.x - b.z, b.y - b.z)
	level_label.size = Vector2(b.z, b.z) * 2
	add_child(level_label)

## A value bar filling one of the plate's channels (the plate is its frame).
func _tube(anchor: String, color: Color) -> ProgressBar:
	var r := rect(anchor)
	var p := UiKit.value_bar(color, int(r.size.y))
	if available():
		p.add_theme_stylebox_override("background", StyleBoxEmpty.new())
		var fill := UiKit.bar_fill(color)
		if fill is NineBox:
			fill = (fill as NineBox).copy()
			(fill as NineBox).inset = 1.0
		p.add_theme_stylebox_override("fill", fill)
	var cap: Label = p.get_meta("caption")
	cap.add_theme_font_size_override("font_size", maxi(10, int(r.size.y * 0.66)))
	p.position = r.position + Vector2(1, 1)
	p.size = r.size - Vector2(2, 2)
	p.custom_minimum_size = p.size
	add_child(p)
	return p

func _draw() -> void:
	if available():
		super._draw()
		return
	# code-drawn plate when the kit is missing
	var pb := UiKit.fancy_panel(0)
	var body := Rect2(Vector2(200, 44) * scale_k, Vector2(560, 168) * scale_k)
	pb.draw(get_canvas_item(), body)
	for k in ["name", "hp", "xp"]:
		UiKit.fancy_inset(0).draw(get_canvas_item(), rect(k).grow(2))
	var o := circle("orb")
	draw_circle(Vector2(o.x, o.y), o.z + 8, UiKit.OUTLINE)
	var b := circle("badge")
	draw_circle(Vector2(b.x, b.y), b.z + 3, Color("a57d4c"))
	draw_circle(Vector2(b.x, b.y), b.z, Color("1a1614"))
