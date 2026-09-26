extends Node
## Player-customisable HUD. Every HUD element is placed by its centre as a
## fraction of the safe screen area (0..1), with a scale, visibility and,
## for groups of buttons, a direction (row or column). A player keeps up to
## three layouts ("slots") and switches between them. Layouts are saved on
## the device (instant) and on the server (they follow the Telegram account
## to other devices).

signal changed

const SAVE_PATH := "user://hud_layout.json"
const SLOTS := 3

## Element ids, their names in the editor, and the default layout.
const NAMES := {
	"profile": "Portrait & health", "level": "Level & XP", "wallet": "Currencies",
	"minimap": "Minimap", "menu": "Menu", "joystick": "Joystick", "attack": "Attack",
	"use": "Use", "target": "Target", "log": "Messages",
}
const DEFAULTS := {
	"profile": {"x": 0.15, "y": 0.08, "s": 1.0, "v": true},
	"level": {"x": 0.17, "y": 0.225, "s": 1.0, "v": true},
	"wallet": {"x": 0.79, "y": 0.05, "s": 1.0, "v": true},
	"minimap": {"x": 0.92, "y": 0.28, "s": 1.0, "v": true},
	"menu": {"x": 0.5, "y": 0.93, "s": 1.0, "v": true, "d": "row"},
	"joystick": {"x": 0.11, "y": 0.77, "s": 1.0, "v": true},
	"attack": {"x": 0.91, "y": 0.78, "s": 1.0, "v": true},
	"use": {"x": 0.78, "y": 0.9, "s": 1.0, "v": true},
	"target": {"x": 0.5, "y": 0.16, "s": 1.0, "v": true},
	"log": {"x": 0.5, "y": 0.32, "s": 1.0, "v": true},
}

var active := 1
var slots := {}

func _ready() -> void:
	reload()

func _defaults() -> Dictionary:
	return DEFAULTS.duplicate(true)

## Re-reads the locally saved layouts (discarding unsaved edits).
func reload() -> void:
	slots = {}
	for i in range(1, SLOTS + 1):
		slots[i] = _defaults()
	if FileAccess.file_exists(SAVE_PATH):
		_merge(JSON.parse_string(FileAccess.get_file_as_string(SAVE_PATH)))

func _merge(data) -> void:
	if not data is Dictionary:
		return
	if data.has("slots") and data.slots is Dictionary:
		active = clampi(int(data.get("active", 1)), 1, SLOTS)
		for k in data.slots.keys():
			var i := int(k)
			if i >= 1 and i <= SLOTS:
				_merge_slot(slots[i], data.slots[k])
	elif data.get("landscape") is Dictionary:
		_merge_slot(slots[1], data.landscape) # layouts saved by older clients

func _merge_slot(dst: Dictionary, src) -> void:
	if not src is Dictionary:
		return
	for id in DEFAULTS.keys():
		var e = src.get(id)
		if not e is Dictionary:
			continue
		var cur: Dictionary = dst[id]
		cur.x = clampf(float(e.get("x", cur.x)), 0.0, 1.0)
		cur.y = clampf(float(e.get("y", cur.y)), 0.0, 1.0)
		cur.s = clampf(float(e.get("s", cur.s)), 0.5, 2.0)
		cur.v = bool(e.get("v", cur.v))
		if cur.has("d"):
			cur.d = "col" if str(e.get("d", cur.d)) == "col" else "row"

## Pulls the server copy (called after login).
func load_remote() -> void:
	var r := await Api.get_json("/v1/settings/hud")
	if r.ok and r.data is Dictionary and r.data.get("value") is Dictionary:
		_merge(r.data.value)
		_save_local()
		changed.emit()

func get_entry(id: String) -> Dictionary:
	return slots[active][id]

func use_slot(i: int) -> void:
	active = clampi(i, 1, SLOTS)
	_save_local()
	changed.emit()
	Api.put_json("/v1/settings/hud", _data())

func reset() -> void:
	slots[active] = _defaults()
	changed.emit()

func save() -> void:
	_save_local()
	changed.emit()
	await Api.put_json("/v1/settings/hud", _data())

func _data() -> Dictionary:
	var out := {}
	for i in slots.keys():
		out[str(i)] = slots[i]
	return {"version": 2, "active": active, "slots": out}

func _save_local() -> void:
	var f := FileAccess.open(SAVE_PATH, FileAccess.WRITE)
	if f:
		f.store_string(JSON.stringify(_data()))
