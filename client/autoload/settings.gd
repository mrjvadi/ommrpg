extends Node
## Player preferences stored on this device (user://settings.json).

const PATH := "user://settings.json"

## Draw the landscape game rotated when a phone is held upright.
var auto_rotate := true
var haptics := true

func _ready() -> void:
	if FileAccess.file_exists(PATH):
		var d = JSON.parse_string(FileAccess.get_file_as_string(PATH))
		if d is Dictionary:
			auto_rotate = bool(d.get("auto_rotate", auto_rotate))
			haptics = bool(d.get("haptics", haptics))
	Telegram.set_haptics(haptics)

func save() -> void:
	Telegram.set_haptics(haptics)
	var f := FileAccess.open(PATH, FileAccess.WRITE)
	if f:
		f.store_string(JSON.stringify({"auto_rotate": auto_rotate, "haptics": haptics}))
