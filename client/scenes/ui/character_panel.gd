extends Modal
## Hero sheet: attributes (spend free points), derived stats, class and the
## faint omen of the hidden Root.

var _pending := {"str": 0, "agi": 0, "int": 0, "vit": 0}

func _init() -> void:
	super("Hero")

func _ready() -> void:
	super()
	Game.character_changed.connect(_render)
	await Game.refresh_character()
	_render()

func _render() -> void:
	if not is_inside_tree():
		return
	clear_body()
	var c := Game.character
	var cls := str(c.get("class", ""))
	body.add_child(UiKit.para("%s - Level %d %s" % [str(c.name), int(c.level), cls.capitalize() if cls != "" else "(class awakens at level 10)"], 20, UiKit.TEXT, true))
	body.add_child(UiKit.label("XP %d / %d" % [int(c.xp), int(c.xp_to_next)], 16, UiKit.MUTED))
	if str(c.get("root_hint", "")) != "":
		body.add_child(UiKit.para(str(c.root_hint), 16, Color("b4a0ff")))
	var free := int(c.get("free_points", 0)) - _spent()
	body.add_child(UiKit.label("Free points: %d" % free, 18, UiKit.ACCENT, true))
	var attrs: Dictionary = c.attributes
	for pair in [["str", "Strength"], ["agi", "Agility"], ["int", "Intellect"], ["vit", "Vitality"]]:
		var row := HBoxContainer.new()
		var lbl := UiKit.label("%s  %d%s" % [pair[1], int(attrs[pair[0]]), ("  +%d" % _pending[pair[0]]) if _pending[pair[0]] > 0 else ""], 18)
		lbl.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(lbl)
		var plus := UiKit.button("+", func():
			_pending[pair[0]] += 1
			_render(), 40)
		plus.custom_minimum_size.x = 56
		plus.disabled = free <= 0
		row.add_child(plus)
		body.add_child(row)
	if _spent() > 0:
		var row := HBoxContainer.new()
		row.add_child(UiKit.button("Confirm", _confirm))
		row.add_child(UiKit.button("Undo", func():
			_pending = {"str": 0, "agi": 0, "int": 0, "vit": 0}
			_render()))
		body.add_child(row)
	var d = c.get("derived")
	if d is Dictionary:
		body.add_child(UiKit.label("Combat", 20, UiKit.TEXT, true))
		for line in [
			"Health %d" % int(d.max_hp), "Attack %d (%s)" % [int(d.attack), str(d.weapon_kind)],
			"Defense %d" % int(d.defense), "Crit %.1f%%" % (float(d.crit_chance) * 100.0),
			"Attack speed %.2fs  Range %.1f" % [float(d.cooldown), float(d.range)], "Move speed %.1f" % float(d.move_speed)]:
			body.add_child(UiKit.label(line, 16, UiKit.MUTED))

func _spent() -> int:
	return int(_pending["str"]) + int(_pending["agi"]) + int(_pending["int"]) + int(_pending["vit"])

func _confirm() -> void:
	var r := await Api.post_json("/v1/character/allocate", _pending)
	_pending = {"str": 0, "agi": 0, "int": 0, "vit": 0}
	if r.ok:
		Game.set_character(r.data)
	else:
		Game.toast(r.error, UiKit.DANGER)
		_render()
