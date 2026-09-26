class_name Modal
extends Control
## Full-screen dimmed modal with a responsive panel, title and close button.

signal closed

var body: VBoxContainer
var title_label: Label
var _panel: PanelContainer
var _scroll: ScrollContainer

func _init(title: String) -> void:
	UiKit.full_rect(self)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var dim := ColorRect.new()
	dim.color = Color(0, 0, 0, 0.6)
	add_child(UiKit.full_rect(dim))
	var center := CenterContainer.new()
	add_child(UiKit.full_rect(center))
	_panel = UiKit.panel()
	center.add_child(_panel)
	var col := VBoxContainer.new()
	_panel.add_child(col)
	var head := HBoxContainer.new()
	col.add_child(head)
	title_label = UiKit.label(title, 26, UiKit.ACCENT, true)
	title_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(title_label)
	head.add_child(UiKit.button("Close", close, 40))
	_scroll = ScrollContainer.new()
	_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	col.add_child(_scroll)
	body = VBoxContainer.new()
	body.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_scroll.add_child(body)

func _ready() -> void:
	get_viewport().size_changed.connect(_fit)
	_fit()

func _fit() -> void:
	var vs := get_viewport_rect().size
	var ins := Telegram.insets_for(get_viewport())
	var w := minf(vs.x - 24.0, 640.0)
	var h := vs.y - ins.x - ins.z - 40.0
	_panel.custom_minimum_size = Vector2(w, 0)
	_scroll.custom_minimum_size = Vector2(w - 30.0, maxf(200.0, h - 100.0))

func close() -> void:
	closed.emit()
	queue_free()

func clear_body() -> void:
	for c in body.get_children():
		c.queue_free()
