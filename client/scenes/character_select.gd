extends Control
## Lists the account's characters and creates new ones. New characters get
## a seed-generated LPC appearance; "Randomise" rolls a new seed.

signal play

const MAX_CHARACTERS := 4

var _list: VBoxContainer
var _status: Label
var _preview: CharacterPreview
var _name: LineEdit
var _recipe := {}
var _seed := 0
var _body := "male"

func _ready() -> void:
	UiKit.full_rect(self)
	var bg := ColorRect.new()
	bg.color = UiKit.BG
	add_child(UiKit.full_rect(bg))
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	add_child(UiKit.full_rect(scroll))
	var margin := MarginContainer.new()
	margin.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	for side in ["left", "right", "top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side, 18)
	scroll.add_child(margin)
	var col := VBoxContainer.new()
	col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	margin.add_child(col)
	col.add_child(UiKit.label("Your characters", 30, UiKit.ACCENT, true))
	_list = VBoxContainer.new()
	col.add_child(_list)
	_status = UiKit.para("", 18, UiKit.MUTED)
	col.add_child(_status)
	col.add_child(_creator())
	_seed = randi()
	_reroll()
	_refresh()

func _refresh() -> void:
	for c in _list.get_children():
		c.queue_free()
	var r := await Api.get_json("/v1/me")
	if not r.ok:
		_status.text = r.error
		return
	Game.characters = r.data.characters
	if Game.characters.is_empty():
		_list.add_child(UiKit.para("No characters yet. Create one below.", 18, UiKit.MUTED))
	for ch in Game.characters:
		_list.add_child(_card(ch))
	if Cfg.autotest:
		await get_tree().create_timer(2.5).timeout
		await Cfg.shot("01_select")
		if Game.characters.is_empty():
			_name.text = "auto%d" % (randi() % 100000)
			await _create()
		elif not Game.characters.is_empty():
			_play(Game.characters[0])

func _card(ch: Dictionary) -> Control:
	var p := UiKit.panel()
	var row := HBoxContainer.new()
	p.add_child(row)
	var pv := CharacterPreview.new()
	pv.custom_minimum_size = Vector2(96, 96)
	pv.show_recipe(ch.appearance)
	row.add_child(pv)
	var info := VBoxContainer.new()
	info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	info.add_child(UiKit.label(str(ch.name), 24, UiKit.TEXT, true))
	var cls := str(ch.get("class", ""))
	info.add_child(UiKit.label("Level %d %s" % [int(ch.level), cls.capitalize() if cls != "" else "Wanderer"], 18, UiKit.MUTED))
	row.add_child(info)
	var btn := UiKit.button("Play", func(): _play(ch))
	btn.custom_minimum_size = Vector2(96, 56)
	row.add_child(btn)
	return p

func _creator() -> Control:
	var p := UiKit.panel()
	var col := VBoxContainer.new()
	p.add_child(col)
	col.add_child(UiKit.label("New character", 24, UiKit.ACCENT, true))
	col.add_child(UiKit.para("Your look is woven from a random seed. Your hidden Root and talents are too - you will discover them by playing.", 16, UiKit.MUTED))
	_preview = CharacterPreview.new()
	_preview.custom_minimum_size = Vector2(160, 160)
	_preview.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	col.add_child(_preview)
	var bodies := HBoxContainer.new()
	bodies.alignment = BoxContainer.ALIGNMENT_CENTER
	for b in ["male", "female"]:
		bodies.add_child(UiKit.button(b.capitalize(), func():
			_body = b
			_reroll(false)))
	bodies.add_child(UiKit.button("Randomise", func(): _reroll()))
	col.add_child(bodies)
	_name = LineEdit.new()
	_name.placeholder_text = "Name (3-16 letters)"
	_name.max_length = 16
	_name.custom_minimum_size = Vector2(0, 52)
	col.add_child(_name)
	col.add_child(UiKit.button("Create", _create, 56))
	return p

func _reroll(new_seed := true) -> void:
	if new_seed:
		_seed = randi()
	var r := await Api.get_json("/v1/sprites/random?seed=%d&body=%s" % [_seed, _body])
	if r.ok:
		_recipe = r.data.recipe
		_preview.show_recipe(_recipe)

func _create() -> void:
	if Game.characters.size() >= MAX_CHARACTERS:
		_status.text = "You can have at most %d characters." % MAX_CHARACTERS
		return
	_status.text = "Creating..."
	var body := {"name": _name.text}
	if not _recipe.is_empty():
		body["appearance"] = _recipe
	var r := await Api.post_json("/v1/characters", body)
	if not r.ok:
		_status.text = r.error
		return
	_status.text = ""
	_name.text = ""
	_reroll()
	_refresh()

func _play(ch: Dictionary) -> void:
	_status.text = "Entering the world..."
	var r := await Api.post_json("/v1/session/character", {"character_id": ch.id})
	if not r.ok:
		_status.text = r.error
		return
	Api.token = r.data.token
	Game.set_character(r.data.character)
	Game.world = r.data.world
	Game.position = r.data.position
	Game.realtime = r.data.realtime
	Game.tileset_url = r.data.tileset
	play.emit()
