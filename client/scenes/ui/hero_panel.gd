extends Modal
## The hero screen: the character on a stage between its equipment slots
## (left), and on the right the level ribbon, combat stats, attributes (spend
## free points) or the bag. Tapping any item opens its card with equip,
## enhance and salvage.

const LEFT_SLOTS := [["head", "head"], ["chest", "chest"], ["hands", "hands"], ["legs", "legs"], ["feet", "feet"]]
const RIGHT_SLOTS := [["weapon", "weapon"], ["offhand", "offhand"], ["ring", "ring"], ["amulet", "amulet"]]
const STAT_NAMES := {
	"damage": "Damage", "armor": "Armor", "str": "Strength", "agi": "Agility", "int": "Intellect",
	"vit": "Vitality", "hp": "Health", "magic": "Spell power", "crit_pct": "Crit %",
	"speed_pct": "Move speed %", "lifesteal_pct": "Life steal %", "xp_pct": "XP bonus %", "magic_find_pct": "Magic find %",
	"elem_dmg": "Elemental damage",
}
const ATTRS := [["str", "Strength", "fist"], ["agi", "Agility", "feet"], ["int", "Intellect", "book"], ["vit", "Vitality", "heart"]]

var tab := "stats"
var _items: Array = []
var _slots := {} # slot -> ItemSlot
var _doll: Control
var _preview: CharacterPreview
var _right: VBoxContainer
var _content: Control
var _pending := {"str": 0, "agi": 0, "int": 0, "vit": 0}
var _busy := false

func _init(start_tab := "stats") -> void:
	super("Hero")
	tab = start_tab
	scrollable = false
	max_size = Vector2(1120, 640)

func _ready() -> void:
	super()
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 14)
	row.size_flags_vertical = Control.SIZE_EXPAND_FILL
	body.add_child(row)
	_doll = _build_doll()
	row.add_child(_doll)
	_right = VBoxContainer.new()
	_right.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_right.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_right.add_theme_constant_override("separation", 10)
	row.add_child(_right)
	Game.character_changed.connect(_render_right)
	Game.wallet_changed.connect(func(): if tab == "bag": _render_right())
	_render_right()
	refresh()
	Game.refresh_character()

func refresh() -> void:
	var inv := await Game.refresh_inventory()
	if inv.is_empty() or not is_inside_tree():
		return
	_items = inv.items
	for s in _slots.keys():
		_slots[s].set_item({})
	for it in _items:
		var eq := str(it.get("equipped", ""))
		if eq != "" and _slots.has(eq):
			_slots[eq].set_item(it)
	_render_right()

# ---------------------------------------------------------------- the doll

func _build_doll() -> Control:
	var frame := PanelContainer.new()
	frame.add_theme_stylebox_override("panel", UiKit.inset_box(0))
	frame.size_flags_vertical = Control.SIZE_EXPAND_FILL
	frame.custom_minimum_size = Vector2(360, 0)
	frame.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	frame.size_flags_stretch_ratio = 0.9
	frame.clip_contents = true
	var area := Control.new()
	area.clip_contents = true
	frame.add_child(area)
	area.add_child(UiKit.full_rect(Stage.new()))
	_preview = CharacterPreview.new()
	_preview.anim = "walk"
	area.add_child(_preview)
	_preview.show_recipe(Game.character.appearance)
	var name_box := VBoxContainer.new()
	name_box.add_theme_constant_override("separation", -4)
	var nm := UiKit.label(str(Game.character.get("name", "")), 22, UiKit.TEXT, true, 6)
	nm.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	name_box.add_child(nm)
	var cls := str(Game.character.get("class", ""))
	var cl := UiKit.label(cls.capitalize() if cls != "" else "Wanderer", 14, UiKit.GOLD, true, 4)
	cl.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	name_box.add_child(cl)
	area.add_child(name_box)
	for pair in LEFT_SLOTS + RIGHT_SLOTS:
		var s := ItemSlot.new(80, pair[1])
		var slot: String = pair[0]
		s.pressed.connect(func(): _open_card(s.item) if not s.item.is_empty() else _show_bag_for(slot))
		area.add_child(s)
		_slots[slot] = s
	area.resized.connect(func(): _layout_doll(area, name_box))
	return frame

func _layout_doll(area: Control, name_box: Control) -> void:
	var sz := area.size
	var gap := 8.0
	var slot := clampf(minf((sz.y - 16 - gap * 4) / 5.0, sz.x * 0.19), 52.0, 84.0)
	var top := (sz.y - (slot * 5 + gap * 4)) / 2
	for i in LEFT_SLOTS.size():
		var s: ItemSlot = _slots[LEFT_SLOTS[i][0]]
		s.custom_minimum_size = Vector2(slot, slot)
		s.size = Vector2(slot, slot)
		s.position = Vector2(10, top + i * (slot + gap))
	var rtop := (sz.y - (slot * 4 + gap * 3)) / 2
	for i in RIGHT_SLOTS.size():
		var s: ItemSlot = _slots[RIGHT_SLOTS[i][0]]
		s.custom_minimum_size = Vector2(slot, slot)
		s.size = Vector2(slot, slot)
		s.position = Vector2(sz.x - slot - 10, rtop + i * (slot + gap))
	var pw := minf(sz.x - slot * 2 - 40, sz.y * 0.78)
	_preview.size = Vector2(pw, pw)
	_preview.position = Vector2((sz.x - pw) / 2, sz.y * 0.86 - pw)
	name_box.size = Vector2(sz.x - slot * 2 - 30, 50)
	name_box.position = Vector2(slot + 15, sz.y - 54)

# ---------------------------------------------------------------- right side

func _render_right() -> void:
	if not is_inside_tree() or _right == null:
		return
	for c in _right.get_children():
		c.queue_free()
	var c := Game.character
	var tabs := UiKit.tabs([["stats", "Stats", "hero"], ["bag", "Bag (%d)" % _bag_items().size(), "bag"]], tab, func(id):
		tab = id
		_render_right(), 44)
	_right.add_child(tabs)
	# level ribbon with the experience bar
	var lvl := Control.new()
	lvl.custom_minimum_size = Vector2(0, 50)
	var rib := UiKit.banner(Vector2(300, 50))
	lvl.add_child(UiKit.full_rect(rib))
	var ll := UiKit.label("Lv. %d" % int(c.get("level", 1)), 22, UiKit.TEXT, true, 6)
	ll.position = Vector2(40, 6)
	lvl.add_child(ll)
	var xp := UiKit.value_bar(Color("3a8fe0"), 24)
	UiKit.set_bar(xp, int(c.get("xp", 0)), max(1, int(c.get("xp_to_next", 1))), "%s / %s" % [UiKit.num(c.get("xp", 0)), UiKit.num(c.get("xp_to_next", 1))])
	lvl.add_child(xp)
	lvl.resized.connect(func():
		xp.position = Vector2(130, 9)
		xp.size = Vector2(maxf(80, lvl.size.x - 175), 24))
	_right.add_child(lvl)
	_content = VBoxContainer.new()
	_content.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_content.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	if tab == "bag":
		_right.add_child(_content)
		_render_bag()
	else:
		# stats can be taller than a short phone screen: let them scroll
		var scroll := ScrollContainer.new()
		scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
		scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
		_right.add_child(scroll)
		scroll.add_child(_content)
		_render_stats()

func _render_stats() -> void:
	var c := Game.character
	var d = c.get("derived")
	var grid_box := UiKit.inset(12)
	_content.add_child(grid_box)
	var grid := GridContainer.new()
	grid.columns = 2
	grid.add_theme_constant_override("h_separation", 18)
	grid.add_theme_constant_override("v_separation", 6)
	grid_box.add_child(grid)
	if d is Dictionary:
		var rows := [
			["HP", UiKit.num(d.max_hp)], ["ATK", UiKit.num(d.attack)],
			["DEF", UiKit.num(d.defense)], ["Crit", "%.1f%%" % (float(d.crit_chance) * 100.0)],
			["Attack speed", "%.2fs" % float(d.cooldown)], ["Range", "%.1f" % float(d.range)],
			["Move speed", "%.1f" % float(d.move_speed)], ["Life steal", "%.1f%%" % (float(d.get("lifesteal", 0)) * 100.0)],
		]
		if str(d.get("element", "")) != "":
			rows.append([str(d.element).capitalize(), "+%d" % int(d.get("element_damage", 0))])
			rows.append(["XP bonus", "%.0f%%" % (float(d.get("xp_bonus", 0)) * 100.0)])
		for r in rows:
			var sr := UiKit.stat_row(r[0], r[1])
			sr.custom_minimum_size.x = 165
			sr.size_flags_horizontal = Control.SIZE_EXPAND_FILL
			grid.add_child(sr)
	# attributes
	var free := int(c.get("free_points", 0)) - _spent()
	var head := HBoxContainer.new()
	head.add_child(UiKit.heading("Attributes", 18))
	head.add_child(UiKit.hspacer())
	head.add_child(UiKit.label("Free points: %d" % free, 18, UiKit.GOOD if free > 0 else UiKit.MUTED))
	_content.add_child(head)
	var attrs: Dictionary = c.get("attributes", {})
	var ag := GridContainer.new()
	ag.columns = 2
	ag.add_theme_constant_override("h_separation", 16)
	_content.add_child(ag)
	for a in ATTRS:
		var key: String = a[0]
		var cell := HBoxContainer.new()
		cell.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		cell.add_child(UiKit.icon(a[2], 28, "bronze"))
		var val := "%d" % int(attrs.get(key, 0))
		if int(_pending[key]) > 0:
			val += "  +%d" % int(_pending[key])
		var l := UiKit.label("%s  %s" % [a[1], val], 18, UiKit.TEXT if int(_pending[key]) == 0 else UiKit.GOOD)
		l.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		cell.add_child(l)
		var plus := UiKit.button("+", func():
			_pending[key] += 1
			_render_right(), 38, "orange")
		plus.custom_minimum_size.x = 46
		plus.add_theme_font_size_override("font_size", 26)
		plus.disabled = free <= 0
		cell.add_child(plus)
		ag.add_child(cell)
	if _spent() > 0:
		var row := HBoxContainer.new()
		var ok := UiKit.button("Confirm", _confirm, 44, "green")
		ok.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(ok)
		var undo := UiKit.button("Undo", func():
			_pending = {"str": 0, "agi": 0, "int": 0, "vit": 0}
			_render_right(), 44, "gray")
		undo.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(undo)
		_content.add_child(row)
	if str(c.get("root_hint", "")) != "":
		_content.add_child(UiKit.para(str(c.root_hint), 15, Color("c8b4ff")))
	elif str(c.get("class", "")) == "":
		_content.add_child(UiKit.para("Your class awakens at level 10 from how you play.", 15, UiKit.MUTED))

func _bag_items() -> Array:
	return _items.filter(func(i): return str(i.get("equipped", "")) == "")

func _render_bag() -> void:
	var head := HBoxContainer.new()
	var gp := UiKit.pill("gold", "gold")
	gp.set_value(UiKit.num(Game.wallet.get("gold", 0)))
	head.add_child(gp)
	var ep := UiKit.pill("essence", "purple")
	ep.set_value(UiKit.num(Game.wallet.get("essence", 0)))
	head.add_child(ep)
	_content.add_child(head)
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_content.add_child(scroll)
	var grid := GridContainer.new()
	grid.add_theme_constant_override("h_separation", 8)
	grid.add_theme_constant_override("v_separation", 8)
	scroll.add_child(grid)
	var bag := _bag_items()
	bag.sort_custom(func(a, b): return Sprites.rarity_index(str(a.item.rarity)) > Sprites.rarity_index(str(b.item.rarity)))
	for it in bag:
		var s := ItemSlot.new(78)
		s.set_item(it)
		s.pressed.connect(func(): _open_card(it))
		grid.add_child(s)
	if bag.is_empty():
		_content.add_child(UiKit.para("Your bag is empty. Hunt monsters and open chests to find gear.", 16, UiKit.MUTED))
	scroll.resized.connect(func(): grid.columns = maxi(1, int((scroll.size.x - 12) / 86.0)))

func _show_bag_for(slot: String) -> void:
	tab = "bag"
	_render_right()
	var n := _bag_items().filter(func(i): return str(i.item.slot) == slot).size()
	Game.toast("%d item(s) for %s in your bag" % [n, slot], UiKit.MUTED)

func _spent() -> int:
	return int(_pending["str"]) + int(_pending["agi"]) + int(_pending["int"]) + int(_pending["vit"])

func _confirm() -> void:
	var r := await Api.post_json("/v1/character/allocate", _pending)
	_pending = {"str": 0, "agi": 0, "int": 0, "vit": 0}
	if r.ok:
		Game.set_character(r.data)
		Telegram.haptic("success")
	else:
		Game.toast(r.error, UiKit.DANGER)
		_render_right()

# ---------------------------------------------------------------- item card

func _open_card(it: Dictionary) -> void:
	var card := ItemCard.new(it)
	card.action.connect(func(kind): _do(kind, it))
	Popups.open(card)

func _do(kind: String, it: Dictionary) -> void:
	if _busy:
		return
	_busy = true
	match kind:
		"equip":
			var r := await Api.post_json("/v1/inventory/%s/equip" % str(it.id))
			if r.ok:
				Game.toast("Equipped " + str(it.display_name), UiKit.GOOD)
			else:
				Game.toast(r.error, UiKit.DANGER)
		"unequip":
			var r := await Api.post_json("/v1/inventory/unequip", {"slot": str(it.equipped)})
			if not r.ok:
				Game.toast(r.error, UiKit.DANGER)
		"enhance":
			var r := await Api.post_json("/v1/inventory/%s/enhance" % str(it.id), {"request_id": Api.request_id()})
			if not r.ok:
				Game.toast(r.error, UiKit.DANGER)
			else:
				var res: Dictionary = r.data.result
				if res.success:
					Game.toast("Success! Now +%d" % int(res.to), UiKit.GOOD)
					Telegram.haptic("success")
				else:
					Game.toast("The enhancement failed (+%d)" % int(res.to), UiKit.DANGER)
					Telegram.haptic("error")
		"salvage":
			if await Popups.confirm("Salvage?", "%s turns into %d essence and %d gold." % [str(it.display_name), int(it.salvage_essence), int(it.salvage_gold)], "Salvage", "red"):
				var r := await Api.post_json("/v1/inventory/%s/salvage" % str(it.id), {"request_id": Api.request_id()})
				if r.ok:
					Game.toast("Salvaged for %d essence" % int(r.data.essence), UiKit.ESSENCE)
				else:
					Game.toast(r.error, UiKit.DANGER)
	_busy = false
	Game.refresh_character()
	await refresh()
	if kind == "enhance":
		for i in _items:
			if i.id == it.id:
				_open_card(i)


class ItemCard extends Modal:
	## Details of one item and what can be done with it.
	signal action(kind: String)
	var it: Dictionary

	func _init(item: Dictionary) -> void:
		super(str(item.item.get("slot", "Item")).capitalize())
		it = item
		max_size = Vector2(620, 560)
		fit_content = true

	func _ready() -> void:
		super()
		var rar := str(it.item.rarity)
		var top := HBoxContainer.new()
		top.add_theme_constant_override("separation", 14)
		body.add_child(top)
		var s := ItemSlot.new(104)
		s.set_item(it)
		s.disabled = true
		top.add_child(s)
		var col := VBoxContainer.new()
		col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		col.add_theme_constant_override("separation", 2)
		top.add_child(col)
		var nm := UiKit.para(str(it.display_name), 22, Sprites.rarity_color(rar), true)
		col.add_child(nm)
		col.add_child(UiKit.label("%s %s  ·  item level %d" % [rar.capitalize(), str(it.item.slot), int(it.item.item_level)], 15, UiKit.MUTED))
		var st: Dictionary = it.state
		var lvl := UiKit.value_bar(Color("3a8fe0"), 20)
		if int(it.xp_to_next) > 0:
			UiKit.set_bar(lvl, int(st.xp), int(it.xp_to_next), "Lv. %d   %d / %d xp" % [int(st.level), int(st.xp), int(it.xp_to_next)])
		else:
			UiKit.set_bar(lvl, 1, 1, "Lv. %d  (max)" % int(st.level))
		col.add_child(lvl)
		var stats := UiKit.inset(10)
		body.add_child(stats)
		var sc := VBoxContainer.new()
		sc.add_theme_constant_override("separation", 3)
		stats.add_child(sc)
		for k in it.stats.keys():
			sc.add_child(UiKit.stat_row(STAT_NAMES.get(k, k), str(it.stats[k]), UiKit.GOOD))
		var el := str(it.item.get("element", ""))
		if el != "":
			body.add_child(UiKit.para("%s element: bonus damage that ignores armour, strong or weak against different monster families." % el.capitalize(), 15, LpcSprite.ELEMENT_COLORS.get(el, Color.WHITE)))
		if str(it.item.get("trait", "")) != "":
			body.add_child(UiKit.para(str(it.item.trait), 15, Sprites.rarity_color("mythic")))
		var row := HBoxContainer.new()
		row.add_theme_constant_override("separation", 10)
		body.add_child(row)
		var equipped := str(it.get("equipped", "")) != ""
		_btn(row, "Unequip" if equipped else "Equip", "unequip" if equipped else "equip", "gray" if equipped else "green")
		if float(it.enhance_chance) > 0:
			_btn(row, "Enhance +%d  (%d%%)" % [int(st.enhance) + 1, int(round(float(it.enhance_chance) * 100))], "enhance", "orange")
		if not equipped:
			_btn(row, "Salvage", "salvage", "red")
		if float(it.enhance_chance) > 0:
			body.add_child(UiKit.para("Enhance cost: %d gold, %d essence. Failing above +7 loses a level." % [int(it.enhance_gold), int(it.enhance_essence)], 14, UiKit.MUTED))

	func _btn(row: HBoxContainer, text: String, kind: String, color: String) -> void:
		var b := UiKit.button(text, func():
			close()
			action.emit(kind), 50, color)
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(b)
