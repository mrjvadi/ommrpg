extends Node
## Session-wide game state shared by scenes: the logged-in account, the
## selected character, its world and live vitals, plus UI notifications.

signal character_changed
signal wallet_changed
signal vitals_changed
signal notify(text: String, color: Color)

var account := {}
var characters: Array = []
var character := {}
var world := {}
var position := {}
var realtime := {}
var tileset_url := ""
var wallet := {"gold": 0, "essence": 0}
var hp := 1
var max_hp := 1
var species := {} # int id -> Dictionary

func toast(text: String, color := Color.WHITE) -> void:
	notify.emit(text, color)

func set_character(c: Dictionary) -> void:
	character = c
	if c.has("derived") and c.derived is Dictionary:
		max_hp = int(c.derived.get("max_hp", max_hp))
	character_changed.emit()

func refresh_character() -> void:
	var r := await Api.get_json("/v1/character")
	if r.ok:
		set_character(r.data)

func refresh_inventory() -> Dictionary:
	var r := await Api.get_json("/v1/inventory")
	if r.ok:
		wallet = r.data.wallet
		wallet_changed.emit()
		return r.data
	return {}

func set_vitals(new_hp: int, new_max: int) -> void:
	hp = new_hp
	max_hp = max(1, new_max)
	vitals_changed.emit()

func load_species() -> void:
	var r := await Api.get_json("/v1/worlds/%d/species" % int(world.id))
	if r.ok:
		for s in r.data.species:
			species[int(s.id)] = s

func move_speed() -> float:
	if character.has("derived") and character.derived is Dictionary:
		return float(character.derived.get("move_speed", 4.5))
	return 4.5

func weapon_kind() -> String:
	if character.has("derived") and character.derived is Dictionary:
		return str(character.derived.get("weapon_kind", "melee"))
	return "melee"

func attack_range() -> float:
	if character.has("derived") and character.derived is Dictionary:
		return float(character.derived.get("range", 1.4))
	return 1.4
