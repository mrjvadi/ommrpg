class_name MinimapFrame
extends KitPart
## Round minimap in the kit's jewelled bronze ring with a north marker and
## a zone plaque underneath (the classic MMO corner map).

const FB_SIZE := Vector2(400, 400)
const FB_ANCHORS := {"hole": [200, 200, 150], "zone": [92, 350, 216, 22]}

var map_texture: Texture2D
var zone_label: Label

func _init(k := 0.42) -> void:
	super._init("minimap", k, FB_SIZE, FB_ANCHORS)
	var zr := rect("zone")
	zone_label = UiKit.label("", maxi(12, int(zr.size.y * 0.9)), UiKit.GOLD, true, 3)
	zone_label.add_theme_font_override("font", UiKit.FONT_TITLE)
	zone_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	zone_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	zone_label.position = zr.position - Vector2(0, 6)
	zone_label.size = zr.size + Vector2(0, 6)
	zone_label.clip_text = true
	add_child(zone_label)

func set_map(tex: Texture2D) -> void:
	map_texture = tex
	queue_redraw()

func set_zone(text: String) -> void:
	zone_label.text = text

func _draw() -> void:
	var h := circle("hole")
	var c := Vector2(h.x, h.y)
	draw_circle(c, h.z + 1, Color("0b0908"))
	if map_texture:
		KitPart.draw_circle_texture(self, map_texture, c, h.z)
	# a soft dark rim so the map sinks into the ring
	for i in 6:
		draw_arc(c, h.z - i * 1.5, 0, TAU, 64, Color(0, 0, 0, 0.22 - i * 0.035), 2.0, true)
	if available():
		super._draw()
	else:
		draw_arc(c, h.z + 5, 0, TAU, 64, UiKit.OUTLINE, 9.0, true)
		draw_arc(c, h.z + 5, 0, TAU, 64, Color("a57d4c"), 5.0, true)
		UiKit.fancy_pill().draw(get_canvas_item(), rect("zone").grow(4))
	# the player, as an arrow-like gem in the middle
	draw_circle(c, 5.5, UiKit.OUTLINE)
	draw_circle(c, 4.0, Color("ffe27a"))
