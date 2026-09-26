extends Modal
## Inventory: equipment and bag, item details, equip/unequip, enhancement
## (+N upgrades with success chance), and salvage into essence.

const STAT_NAMES := {
	"damage": "Damage", "armor": "Armor", "str": "Strength", "agi": "Agility", "int": "Intellect",
	"vit": "Vitality", "hp": "Health", "magic": "Spell power", "crit_pct": "Crit %",
	"speed_pct": "Move speed %", "lifesteal_pct": "Life steal %", "xp_pct": "XP bonus %", "magic_find_pct": "Magic find %",
	"elem_dmg": "Elemental damage",
}

var _items: Array = []
var _detail: Control
var _busy := false

func _init() -> void:
	super("Bag")

func _ready() -> void:
	super()
	refresh()

func refresh() -> void:
	var inv := await Game.refresh_inventory()
	if inv.is_empty():
		return
	_items = inv.items
	_render()

func _render() -> void:
	clear_body()
	body.add_child(UiKit.label("Gold %d    Essence %d" % [int(Game.wallet.gold), int(Game.wallet.essence)], 18, UiKit.ACCENT))
	body.add_child(UiKit.label("Equipped", 20, UiKit.TEXT, true))
	body.add_child(_grid(_items.filter(func(i): return str(i.get("equipped", "")) != "")))
	body.add_child(UiKit.label("Bag (%d)" % _items.filter(func(i): return str(i.get("equipped", "")) == "").size(), 20, UiKit.TEXT, true))
	body.add_child(_grid(_items.filter(func(i): return str(i.get("equipped", "")) == "")))
	_detail = VBoxContainer.new()
	body.add_child(_detail)

func _grid(list: Array) -> Control:
	var g := GridContainer.new()
	g.columns = 5
	g.add_theme_constant_override("h_separation", 6)
	g.add_theme_constant_override("v_separation", 6)
	for it in list:
		g.add_child(_slot(it))
	if list.is_empty():
		return UiKit.label("empty", 14, UiKit.MUTED)
	return g

func _slot(it: Dictionary) -> Control:
	var b := Button.new()
	b.custom_minimum_size = Vector2(88, 88)
	var rar := str(it.item.rarity)
	b.add_theme_stylebox_override("normal", UiKit.box(Color("14141f"), 6, 2, Sprites.rarity_color(rar)))
	b.add_theme_stylebox_override("hover", UiKit.box(Color("1d1d2c"), 6, 3, Sprites.rarity_color(rar)))
	b.add_theme_stylebox_override("pressed", UiKit.box(Color("24243a"), 6, 3, UiKit.ACCENT))
	b.expand_icon = true
	b.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
	b.vertical_icon_alignment = VERTICAL_ALIGNMENT_CENTER
	b.pressed.connect(func(): _show(it))
	var st: Dictionary = it.state
	if int(st.enhance) > 0:
		var tag := UiKit.label("+%d" % int(st.enhance), 13, UiKit.ACCENT, true)
		tag.position = Vector2(4, 0)
		b.add_child(tag)
	var lv := UiKit.label("L%d" % int(st.level), 11, UiKit.MUTED)
	lv.position = Vector2(4, 66)
	b.add_child(lv)
	_set_icon(b, it)
	return b

func _set_icon(b: Button, it: Dictionary) -> void:
	var tex := await Sprites.fetch(Sprites.icon_url(it.item))
	if is_instance_valid(b):
		b.icon = tex

func _show(it: Dictionary) -> void:
	for c in _detail.get_children():
		c.queue_free()
	var p := UiKit.panel()
	_detail.add_child(p)
	var col := VBoxContainer.new()
	p.add_child(col)
	var rar := str(it.item.rarity)
	col.add_child(UiKit.para(str(it.display_name), 22, Sprites.rarity_color(rar), true))
	col.add_child(UiKit.label("%s %s - item level %d" % [rar.capitalize(), str(it.item.slot), int(it.item.item_level)], 15, UiKit.MUTED))
	var st: Dictionary = it.state
	var lvl_text := "Level %d" % int(st.level)
	if int(it.xp_to_next) > 0:
		lvl_text += "  (%d / %d xp)" % [int(st.xp), int(it.xp_to_next)]
	else:
		lvl_text += "  (max)"
	col.add_child(UiKit.label(lvl_text, 15, UiKit.TEXT))
	for k in it.stats.keys():
		col.add_child(UiKit.label("%s  %s" % [STAT_NAMES.get(k, k), str(it.stats[k])], 16, UiKit.GOOD))
	if str(it.item.get("element", "")) != "":
		var el := str(it.item.element)
		col.add_child(UiKit.para("%s element: bonus damage, strong or weak against different monster families" % el.capitalize(), 15, LpcSprite.ELEMENT_COLORS.get(el, Color.WHITE)))
	if str(it.item.get("trait", "")) != "":
		col.add_child(UiKit.para(str(it.item.trait), 15, Sprites.rarity_color("mythic")))
	var row := HBoxContainer.new()
	col.add_child(row)
	if str(it.get("equipped", "")) != "":
		row.add_child(UiKit.button("Unequip", func(): _unequip(str(it.equipped))))
	else:
		row.add_child(UiKit.button("Equip", func(): _equip(it)))
	if float(it.enhance_chance) > 0:
		row.add_child(UiKit.button("Enhance +%d (%d%%)" % [int(st.enhance) + 1, int(round(float(it.enhance_chance) * 100))], func(): _enhance(it)))
		col.add_child(UiKit.para("Cost: %d gold, %d essence. Failing above +7 loses a level." % [int(it.enhance_gold), int(it.enhance_essence)], 13, UiKit.MUTED))
	if str(it.get("equipped", "")) == "":
		row.add_child(UiKit.button("Salvage", func(): _salvage(it)))
		col.add_child(UiKit.para("Salvage gives %d essence and %d gold." % [int(it.salvage_essence), int(it.salvage_gold)], 13, UiKit.MUTED))

func _equip(it: Dictionary) -> void:
	if _busy:
		return
	_busy = true
	var r := await Api.post_json("/v1/inventory/%s/equip" % str(it.id))
	_busy = false
	if r.ok:
		Game.toast("Equipped " + str(it.display_name), UiKit.GOOD)
		Game.refresh_character()
		refresh()
	else:
		Game.toast(r.error, UiKit.DANGER)

func _unequip(slot: String) -> void:
	var r := await Api.post_json("/v1/inventory/unequip", {"slot": slot})
	if r.ok:
		Game.refresh_character()
		refresh()

func _enhance(it: Dictionary) -> void:
	if _busy:
		return
	_busy = true
	var r := await Api.post_json("/v1/inventory/%s/enhance" % str(it.id), {"request_id": Api.request_id()})
	_busy = false
	if not r.ok:
		Game.toast(r.error, UiKit.DANGER)
		return
	var res: Dictionary = r.data.result
	if res.success:
		Game.toast("Success! Now +%d" % int(res.to), UiKit.GOOD)
		Telegram.haptic("success")
	else:
		Game.toast("The enhancement failed (+%d)" % int(res.to), UiKit.DANGER)
		Telegram.haptic("error")
	Game.refresh_character()
	await refresh()
	for i in _items:
		if i.id == it.id:
			_show(i)

func _salvage(it: Dictionary) -> void:
	var r := await Api.post_json("/v1/inventory/%s/salvage" % str(it.id), {"request_id": Api.request_id()})
	if r.ok:
		Game.toast("Salvaged for %d essence" % int(r.data.essence), Color("a88cff"))
		refresh()
	else:
		Game.toast(r.error, UiKit.DANGER)
