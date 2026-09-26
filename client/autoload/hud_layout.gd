extends Node
## Player-customisable HUD layout. Every HUD element is placed by its centre
## as a fraction of the safe screen area (0..1), plus a scale and visibility,
## so a layout adapts to any phone size. Portrait and landscape keep separate
## layouts. Layouts are stored locally (instant) and on the server (follows
## the player's Telegram account to other devices).

signal changed

const SAVE_PATH := "user://hud_layout.json"

## Defaults: centre x, centre y (fractions), scale.
const DEFAULTS := {
	"portrait": {
		"vitals": {"x": 0.30, "y": 0.06, "s": 1.0, "v": true},
		"wallet": {"x": 0.80, "y": 0.05, "s": 1.0, "v": true},
		"minimap": {"x": 0.85, "y": 0.16, "s": 1.0, "v": true},
		"menu": {"x": 0.50, "y": 0.97, "s": 1.0, "v": true},
		"joystick": {"x": 0.22, "y": 0.83, "s": 1.0, "v": true},
		"actions": {"x": 0.80, "y": 0.83, "s": 1.0, "v": true},
		"log": {"x": 0.50, "y": 0.22, "s": 1.0, "v": true},
	},
	"landscape": {
		"vitals": {"x": 0.16, "y": 0.09, "s": 1.0, "v": true},
		"wallet": {"x": 0.60, "y": 0.06, "s": 1.0, "v": true},
		"minimap": {"x": 0.91, "y": 0.17, "s": 1.0, "v": true},
		"menu": {"x": 0.50, "y": 0.95, "s": 1.0, "v": true},
		"joystick": {"x": 0.12, "y": 0.76, "s": 1.0, "v": true},
		"actions": {"x": 0.88, "y": 0.76, "s": 1.0, "v": true},
		"log": {"x": 0.50, "y": 0.18, "s": 1.0, "v": true},
	},
}

var layouts := {}

func _ready() -> void:
	reload()

## Re-reads the locally saved layout (discarding unsaved edits).
func reload() -> void:
	layouts = DEFAULTS.duplicate(true)
	if FileAccess.file_exists(SAVE_PATH):
		var data = JSON.parse_string(FileAccess.get_file_as_string(SAVE_PATH))
		_merge(data)

func _merge(data) -> void:
	if not data is Dictionary:
		return
	for orient in DEFAULTS.keys():
		if not data.has(orient) or not data[orient] is Dictionary:
			continue
		for id in DEFAULTS[orient].keys():
			var e = data[orient].get(id)
			if e is Dictionary:
				var cur: Dictionary = layouts[orient][id]
				cur.x = clampf(float(e.get("x", cur.x)), 0.0, 1.0)
				cur.y = clampf(float(e.get("y", cur.y)), 0.0, 1.0)
				cur.s = clampf(float(e.get("s", cur.s)), 0.5, 2.0)
				cur.v = bool(e.get("v", cur.v))

## Pulls the server copy (called after login).
func load_remote() -> void:
	var r := await Api.get_json("/v1/settings/hud")
	if r.ok and r.data is Dictionary and r.data.get("value") is Dictionary:
		_merge(r.data.value)
		_save_local()
		changed.emit()

func orientation(size: Vector2) -> String:
	return "landscape" if size.x > size.y else "portrait"

func get_entry(orient: String, id: String) -> Dictionary:
	return layouts[orient][id]

func set_entry(orient: String, id: String, entry: Dictionary) -> void:
	layouts[orient][id] = entry

func reset(orient: String) -> void:
	layouts[orient] = DEFAULTS[orient].duplicate(true)
	changed.emit()

func save() -> void:
	_save_local()
	changed.emit()
	await Api.put_json("/v1/settings/hud", layouts)

func _save_local() -> void:
	var f := FileAccess.open(SAVE_PATH, FileAccess.WRITE)
	if f:
		f.store_string(JSON.stringify(layouts))
