class_name FloatingText
extends Label
## Damage numbers and small pop-up texts that drift up and fade.

static func spawn(parent: Node, pos: Vector2, text: String, color := Color.WHITE, size := 14) -> void:
	var l := FloatingText.new()
	l.text = text
	l.add_theme_font_size_override("font_size", size)
	l.add_theme_color_override("font_color", color)
	l.add_theme_color_override("font_outline_color", Color.BLACK)
	l.add_theme_constant_override("outline_size", 5)
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.size = Vector2(120, 20)
	l.position = pos - Vector2(60, 40)
	l.z_index = 50
	parent.add_child(l)
	var tw := l.create_tween()
	tw.set_parallel(true)
	tw.tween_property(l, "position:y", l.position.y - 28, 0.9)
	tw.tween_property(l, "modulate:a", 0.0, 0.9).set_delay(0.3)
	tw.chain().tween_callback(l.queue_free)
