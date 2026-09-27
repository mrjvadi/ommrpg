class_name Modal
extends Control
## A game window: dimmed screen, the kit's carved iron frame, a bronze
## crest over an engraved title plaque and a jewel close button. Windows are opened through
## `Popups.open()` so the Telegram Back button, Escape and the popup stack
## all work the same everywhere.
##
## `body` is a VBox inside a scroll area. Screens that lay out their own
## columns set `scrollable = false` before `_ready` and fill `body` directly.

signal closed

var body: VBoxContainer
var title_label: Label
var header: HBoxContainer
var scrollable := true
## Shrink the window's height to its content (dialogs, cards).
var fit_content := false
## Fraction of the screen the window may use.
var max_size := Vector2(1060, 620)
var _panel: PanelContainer
var _scroll: ScrollContainer
var _ribbon: Control
var _crest: TextureRect
var _closing := false

func _init(title: String) -> void:
	UiKit.full_rect(self)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var dim := ColorRect.new()
	dim.color = Color(0, 0, 0, 0.62)
	dim.gui_input.connect(func(e): if e is InputEventMouseButton and e.pressed and e.button_index == MOUSE_BUTTON_LEFT: close())
	add_child(UiKit.full_rect(dim))
	_panel = UiKit.window(16)
	_panel.mouse_filter = Control.MOUSE_FILTER_STOP
	add_child(_panel)
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 10)
	_panel.add_child(col)
	header = HBoxContainer.new()
	header.custom_minimum_size.y = 6
	col.add_child(header)
	title_label = UiKit.title(title, 22)
	title_label.visible = false
	header.add_child(UiKit.hspacer())
	body = VBoxContainer.new()
	body.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	body.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_scroll = ScrollContainer.new()
	_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	col.add_child(_scroll)
	_scroll.add_child(body)
	# the crest, title plaque and close button float over the frame's top edge
	var crest := UiKit.part("crest")
	if crest:
		_crest = TextureRect.new()
		_crest.texture = crest
		_crest.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
		_crest.stretch_mode = TextureRect.STRETCH_SCALE
		_crest.texture_filter = CanvasItem.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS
		_crest.mouse_filter = Control.MOUSE_FILTER_IGNORE
		add_child(_crest)
	var plaque := Pack.nine("plaque", [8, 8, 8, 8], 0.5)
	if plaque:
		# metal title plaque from the GUI kit
		var pl := Panel.new()
		pl.add_theme_stylebox_override("panel", plaque)
		pl.mouse_filter = Control.MOUSE_FILTER_IGNORE
		_ribbon = pl
	else:
		_ribbon = Ribbon.new(Vector2(320, 50))
	add_child(_ribbon)
	_ribbon.add_child(title_label)
	title_label.visible = true
	title_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	var x := UiKit.close_button(close)
	x.name = "Close"
	add_child(x)
	modulate.a = 0.0

func _ready() -> void:
	if not scrollable:
		_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	get_viewport().size_changed.connect(_fit)
	Telegram.insets_changed.connect(_fit)
	_fit()
	body.minimum_size_changed.connect(func(): if fit_content: _fit.call_deferred())
	var tw := create_tween().set_parallel(true)
	tw.tween_property(self, "modulate:a", 1.0, 0.12)
	_panel.pivot_offset = _panel.size / 2
	_panel.scale = Vector2(0.94, 0.94)
	tw.tween_property(_panel, "scale", Vector2.ONE, 0.16).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)

func _fit() -> void:
	var vs := get_viewport_rect().size
	var ins := Telegram.insets_for(get_viewport())
	var top := 50.0 if _crest else 30.0
	var avail := Vector2(vs.x - ins.w - ins.y - 24.0, vs.y - ins.x - ins.z - top - 14.0)
	var sz := Vector2(minf(avail.x, max_size.x), minf(avail.y, max_size.y))
	if fit_content:
		var need := body.get_combined_minimum_size().y + header.get_combined_minimum_size().y + _panel.get_theme_stylebox("panel").get_minimum_size().y + 12.0
		sz.y = minf(sz.y, maxf(200.0, need))
	_panel.custom_minimum_size = sz
	_panel.size = sz
	_panel.position = Vector2(ins.w + (vs.x - ins.w - ins.y - sz.x) / 2, ins.x + top + (avail.y - sz.y) / 2)
	_panel.pivot_offset = sz / 2
	# whatever the frame's padding and the header leave for the body
	var chrome := _panel.get_theme_stylebox("panel").get_minimum_size().y + header.get_combined_minimum_size().y + 10.0
	_scroll.custom_minimum_size = Vector2(0, maxf(40.0, sz.y - chrome))
	var rw := clampf(title_label.get_combined_minimum_size().x + 110.0, 260.0, sz.x * 0.7)
	_ribbon.size = Vector2(rw, 50)
	_ribbon.position = Vector2(_panel.position.x + (sz.x - rw) / 2, _panel.position.y - 22)
	title_label.position = Vector2(0, 3)
	title_label.size = Vector2(rw, 44)
	if _crest:
		var cs := Pack.part_size("crest") * 0.42
		_crest.size = cs
		_crest.position = Vector2(_panel.position.x + (sz.x - cs.x) / 2, _ribbon.position.y + 30 - cs.y)
	var x: Control = get_node("Close")
	x.position = Vector2(_panel.position.x + sz.x - 34, _panel.position.y - 14)

func set_title(t: String) -> void:
	title_label.text = t
	_fit()

func close() -> void:
	if _closing:
		return
	_closing = true
	closed.emit()
	var tw := create_tween()
	tw.tween_property(self, "modulate:a", 0.0, 0.1)
	tw.tween_callback(queue_free)

func clear_body() -> void:
	for c in body.get_children():
		c.queue_free()
