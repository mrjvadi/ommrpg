class_name ItemSlot
extends Button
## An equipment/bag slot: metallic frame in the item's rarity colour, the
## item icon, "Lv. N" at the bottom, "+N" enhancement at the top and an
## element gem. Empty slots show a faded placeholder icon of the slot type.

var item: Dictionary = {}
var placeholder := ""
var _icon := TextureRect.new()
var _ph: UiIcon
var _level: Label
var _plus: Label
var _badge: Control
var _elem := Color(0, 0, 0, 0)

func _init(slot_size := 84.0, placeholder_icon := "") -> void:
	custom_minimum_size = Vector2(slot_size, slot_size)
	placeholder = placeholder_icon
	focus_mode = Control.FOCUS_NONE
	add_theme_stylebox_override("focus", StyleBoxEmpty.new())
	_icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	_icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	_icon.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	_icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_icon)
	if placeholder != "":
		_ph = UiIcon.new(placeholder, slot_size * 0.5, "muted")
		_ph.modulate.a = 0.55
		add_child(_ph)
	_level = UiKit.label("", clampi(int(slot_size * 0.2), 11, 18), UiKit.TEXT, true, 4)
	_level.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	add_child(_level)
	_plus = UiKit.label("", clampi(int(slot_size * 0.19), 11, 17), UiKit.GOLD, true, 4)
	add_child(_plus)
	_badge = UiKit.badge()
	_badge.visible = false
	add_child(_badge)
	resized.connect(_layout)
	set_item({})

func _layout() -> void:
	var s := size
	_icon.position = s * 0.14
	_icon.size = s * 0.72
	if _ph:
		_ph.position = (s - _ph.custom_minimum_size) / 2
		_ph.size = _ph.custom_minimum_size
	_level.position = Vector2(0, s.y - _level.get_combined_minimum_size().y - 3)
	_level.size.x = s.x
	_plus.position = Vector2(7, 2)
	_badge.position = Vector2(s.x - 18, -6)

func set_item(it: Dictionary) -> void:
	item = it
	var rar := "empty"
	if not it.is_empty():
		rar = str(it.item.rarity)
	for st in ["normal", "hover", "pressed", "disabled"]:
		var b := UiKit.slot_box(rar)
		if st == "hover":
			UiKit.tint(b, Color(1.25, 1.2, 1.1))
		elif st == "pressed":
			UiKit.tint(b, Color(1.5, 1.25, 0.7))
		add_theme_stylebox_override(st, b)
	add_theme_stylebox_override("hover_pressed", get_theme_stylebox("pressed"))
	if _ph:
		_ph.visible = it.is_empty()
	_icon.texture = null
	_level.text = ""
	_plus.text = ""
	_elem = Color(0, 0, 0, 0)
	if it.is_empty():
		queue_redraw()
		return
	var st: Dictionary = it.get("state", {})
	_level.text = "Lv. %d" % int(st.get("level", 1))
	if int(st.get("enhance", 0)) > 0:
		_plus.text = "+%d" % int(st.enhance)
	var el := str(it.item.get("element", ""))
	if el != "":
		_elem = LpcSprite.ELEMENT_COLORS.get(el, Color.WHITE)
	_load(it)
	queue_redraw()

func set_badge(on: bool) -> void:
	_badge.visible = on

func _load(it: Dictionary) -> void:
	var tex := await Sprites.fetch(Sprites.icon_url(it.item))
	if is_instance_valid(self) and item == it:
		_icon.texture = tex

func _draw() -> void:
	if _elem.a > 0:
		var c := Vector2(size.x - 13, 13)
		draw_circle(c, 7, UiKit.OUTLINE)
		draw_circle(c, 5, _elem)
		draw_circle(c - Vector2(1.5, 1.5), 1.8, Color(1, 1, 1, 0.7))
