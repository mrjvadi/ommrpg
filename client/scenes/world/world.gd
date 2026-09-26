extends Node
## The playable world. Streams seed-generated terrain around the player,
## predicts movement locally (validated by presence-service), talks to the
## realtime backend through Centrifugo RPC and renders everyone else.

signal exit_to_menu

const TILE := MapView.TILE
const InventoryPanel := preload("res://scenes/ui/inventory_panel.gd")
const CharacterPanel := preload("res://scenes/ui/character_panel.gd")
const HistoryPanel := preload("res://scenes/ui/history_panel.gd")

var map: MapView
var camera := Camera2D.new()
var player: LpcSprite
var hud: Hud
var overlay := CanvasLayer.new()
var modal: Control

var zone := ""
var pos := Vector2.ZERO
var area := Vector2i(-99999, -99999)
var monsters := {} # id -> MonsterNode
var remotes := {} # id -> {node, target, seen}
var loading_chunks := {}
var channels := {} # channel -> true
var ready_to_play := false
var target_id := ""
var path: Array = [] # tiles to auto-walk (tap-to-move)
var path_target := "" # monster to attack when the path ends

var _move_inflight := false
var _move_seq := 0
var _last_sent := Vector2(-1, -1)
var _last_send_t := 0.0
var _next_attack_t := 0.0
var _attack_held := false
var _timers := {"monsters": 0.0, "nearby": 0.0, "vitals": 0.0, "minimap": 0.0, "stream": 0.0}
var _exit_armed := 0.0
var _no_target_t := 0.0
var _autotest_start := 0.0

func _ready() -> void:
	var world_root := Node2D.new()
	add_child(world_root)
	map = MapView.new()
	world_root.add_child(map)
	var hud_layer := CanvasLayer.new()
	hud_layer.layer = 5
	add_child(hud_layer)
	hud = Hud.new()
	hud_layer.add_child(hud)
	overlay.layer = 10
	add_child(overlay)
	hud.attack_pressed.connect(func(): _attack_held = true; _try_attack())
	hud.attack_btn.released.connect(func(): _attack_held = false)
	hud.interact_pressed.connect(_interact)
	hud.menu.connect(_on_menu)
	get_viewport().size_changed.connect(_fit_camera)
	Realtime.publication.connect(_on_publication)
	Realtime.connected.connect(_on_connected)
	Realtime.disconnected.connect(func(reason): Game.toast("Connection lost, reconnecting...", UiKit.DANGER))
	Game.toast("Loading world...", UiKit.MUTED)
	var tex := await Sprites.fetch(Game.tileset_url)
	map.setup(tex)
	player = LpcSprite.new()
	player.set_label(str(Game.character.name), UiKit.ACCENT)
	player.set_recipe(Game.character.appearance)
	player.set_fx(Game.character.appearance, Game.character.get("fx"))
	Game.character_changed.connect(func():
		if is_instance_valid(player):
			player.set_recipe(Game.character.appearance)
			player.set_fx(Game.character.appearance, Game.character.get("fx")))
	map.entities.add_child(player)
	player.add_child(camera)
	camera.position_smoothing_enabled = true
	camera.position_smoothing_speed = 10.0
	_fit_camera()
	await Game.load_species()
	Game.refresh_inventory()
	Realtime.connect_to(Cfg.realtime_url(str(Game.realtime.url)), str(Game.realtime.token))
	_autotest_start = Time.get_ticks_msec() / 1000.0

func _exit_tree() -> void:
	if Realtime.publication.is_connected(_on_publication):
		Realtime.publication.disconnect(_on_publication)
	if Realtime.connected.is_connected(_on_connected):
		Realtime.connected.disconnect(_on_connected)

func _fit_camera() -> void:
	var vs := get_viewport().get_visible_rect().size
	var z := minf(vs.x, vs.y) / (12.0 * TILE)
	z = maxf(1.0, round(z * 2.0) / 2.0)
	camera.zoom = Vector2(z, z)

# ---------------------------------------------------------------- zones

func _on_connected() -> void:
	var r := await Realtime.call_rpc("enter")
	if not r.ok:
		Game.toast("Could not enter the world: " + str(r.error), UiKit.DANGER)
		return
	await _enter_zone(r.data)
	ready_to_play = true
	Game.toast("Welcome to %s" % str(Game.world.get("name", "the world")), UiKit.ACCENT)
	_refresh_vitals()

func _enter_zone(p: Dictionary, floor_data = null) -> void:
	var z := str(p.zone)
	path.clear()
	if z != zone:
		zone = z
		for m in monsters.values():
			m.queue_free()
		monsters.clear()
		for r in remotes.values():
			r.node.queue_free()
		remotes.clear()
		for ch in channels.keys():
			Realtime.unsubscribe(ch)
		channels.clear()
		map.clear()
		loading_chunks.clear()
		area = Vector2i(-99999, -99999)
		if z.begins_with("d:"):
			if floor_data == null:
				var r := await Realtime.call_rpc("dungeon")
				if not r.ok:
					Game.toast(str(r.error), UiKit.DANGER)
					return
				floor_data = r.data.floor
				Game.toast(str(r.data.name), UiKit.ACCENT)
			map.set_floor(floor_data)
			var parts := z.split(":")
			_set_channels(["dungeon:%s_%s" % [parts[1], parts[2]]])
	pos = Vector2(float(p.x), float(p.y))
	player.position = pos * TILE
	camera.reset_smoothing()
	_last_sent = pos
	_refresh_monsters()
	_refresh_nearby()

func _set_channels(wanted: Array) -> void:
	for ch in channels.keys():
		if not ch in wanted:
			Realtime.unsubscribe(ch)
			channels.erase(ch)
	for ch in wanted:
		if not channels.has(ch):
			Realtime.subscribe(ch)
			channels[ch] = true

func _stream() -> void:
	if zone == "" or zone.begins_with("d:"):
		return
	var c := Vector2i(floori(pos.x / MapView.CHUNK), floori(pos.y / MapView.CHUNK))
	var n := int(Game.world.size_chunks)
	for dy in range(-1, 2):
		for dx in range(-1, 2):
			var k := c + Vector2i(dx, dy)
			if k.x < 0 or k.y < 0 or k.x >= n or k.y >= n:
				continue
			if not map.chunks.has(k) and not loading_chunks.has(k):
				_load_chunk(k)
	for k in map.chunks.keys():
		if absi(k.x - c.x) > 2 or absi(k.y - c.y) > 2:
			map.remove_chunk(k)
	if c != area:
		area = c
		var wanted := []
		for dy in range(-1, 2):
			for dx in range(-1, 2):
				wanted.append("area:w%d_%d_%d" % [int(Game.world.id), c.x + dx, c.y + dy])
		_set_channels(wanted)
		_refresh_monsters()

func _load_chunk(k: Vector2i) -> void:
	loading_chunks[k] = true
	var z := zone
	var r := await Api.get_json("/v1/worlds/%d/chunks/%d/%d" % [int(Game.world.id), k.x, k.y])
	loading_chunks.erase(k)
	if r.ok and z == zone and not zone.begins_with("d:"):
		map.add_chunk(r.data)

# ---------------------------------------------------------------- periodic sync

func _refresh_monsters() -> void:
	if not Realtime.is_connected_now:
		return
	var z := zone
	var r := await Realtime.call_rpc("monsters")
	if not r.ok or z != zone or str(r.data.zone) != zone:
		return
	var seen := {}
	for st in r.data.monsters:
		var id := str(st.id)
		seen[id] = true
		if monsters.has(id):
			monsters[id].apply_state(st)
		else:
			var m := MonsterNode.new()
			map.entities.add_child(m)
			m.setup(st, Game.species.get(int(st.spawn.species), {}))
			monsters[id] = m
	for id in monsters.keys():
		if not seen.has(id):
			monsters[id].queue_free()
			monsters.erase(id)

func _refresh_nearby() -> void:
	if not Realtime.is_connected_now:
		return
	var r := await Realtime.call_rpc("nearby")
	if r.ok:
		for p in r.data.players:
			_upsert_remote(str(p.character_id), Vector2(float(p.x), float(p.y)), str(p.get("dir", "down")))

func _refresh_vitals() -> void:
	var r := await Realtime.call_rpc("vitals")
	if r.ok:
		Game.set_vitals(int(r.data.hp), int(r.data.max_hp))

func _upsert_remote(id: String, p: Vector2, dir: String) -> void:
	if id == str(Game.character.id):
		return
	var now := Time.get_ticks_msec() / 1000.0
	if remotes.has(id):
		var r: Dictionary = remotes[id]
		var buf: Array = r.buf
		buf.append([now, p])
		while buf.size() > 12:
			buf.pop_front()
		r.seen = now
		r.dir = dir
		return
	var node := LpcSprite.new()
	node.position = p * TILE
	map.entities.add_child(node)
	remotes[id] = {"node": node, "buf": [[now, p]], "seen": now, "dir": dir}
	_load_remote_look(id)

func _load_remote_look(id: String) -> void:
	var r := await Api.get_json("/v1/characters/%s/public" % id)
	if r.ok and remotes.has(id):
		var node: LpcSprite = remotes[id].node
		node.set_label("%s  %d" % [str(r.data.name), int(r.data.level)])
		node.set_recipe(r.data.appearance)
		node.set_fx(r.data.appearance, r.data.get("fx"))

# ---------------------------------------------------------------- frame

func _process(delta: float) -> void:
	var now := Time.get_ticks_msec() / 1000.0
	for k in _timers.keys():
		_timers[k] -= delta
	if _timers.stream <= 0:
		_timers.stream = 0.25
		_stream()
	if not ready_to_play:
		return
	if _timers.monsters <= 0:
		_timers.monsters = 4.0
		_refresh_monsters()
	if _timers.nearby <= 0:
		_timers.nearby = 5.0
		_refresh_nearby()
	if _timers.vitals <= 0:
		_timers.vitals = 3.0
		_refresh_vitals()
	if _timers.minimap <= 0:
		_timers.minimap = 0.5
		hud.set_minimap(map.minimap_image(pos))
	_move(delta)
	if _attack_held or Input.is_physical_key_pressed(KEY_SPACE):
		_try_attack()
	var near := map.nearest_interactive(pos)
	hud.interact_btn.highlight = not near.is_empty()
	hud.interact_btn.queue_redraw()
	for id in remotes.keys():
		var r: Dictionary = remotes[id]
		var node: LpcSprite = r.node
		var goal: Vector2 = _interpolate(r.buf, now - INTERP_DELAY) * TILE
		var step := goal - node.position
		node.moving = step.length() > 0.4
		if node.moving:
			node.face_towards(step)
		else:
			node.dir = str(r.dir)
		node.position = goal
		if now - float(r.seen) > 100.0:
			node.queue_free()
			remotes.erase(id)
	if Cfg.autotest:
		_autotest(now)

## Remote players are rendered slightly in the past and interpolated
## between received snapshots (entity interpolation), which hides network
## jitter; short gaps are extrapolated from the last velocity.
const INTERP_DELAY := 0.12
const MAX_EXTRAPOLATE := 0.25

func _interpolate(buf: Array, t: float) -> Vector2:
	if buf.size() == 1 or t <= float(buf[0][0]):
		return buf[0][1]
	for i in range(buf.size() - 1, 0, -1):
		var a: Array = buf[i - 1]
		var b: Array = buf[i]
		if t >= float(a[0]) and t <= float(b[0]):
			var span := maxf(0.001, float(b[0]) - float(a[0]))
			return (a[1] as Vector2).lerp(b[1], (t - float(a[0])) / span)
	var last: Array = buf[buf.size() - 1]
	var prev: Array = buf[buf.size() - 2]
	var dt := maxf(0.001, float(last[0]) - float(prev[0]))
	var over := minf(t - float(last[0]), MAX_EXTRAPOLATE)
	if dt > 0.5:
		return last[1]
	return (last[1] as Vector2) + ((last[1] as Vector2) - (prev[1] as Vector2)) / dt * over

func _input_vector() -> Vector2:
	var v := Vector2.ZERO
	if Input.is_physical_key_pressed(KEY_A) or Input.is_physical_key_pressed(KEY_LEFT):
		v.x -= 1
	if Input.is_physical_key_pressed(KEY_D) or Input.is_physical_key_pressed(KEY_RIGHT):
		v.x += 1
	if Input.is_physical_key_pressed(KEY_W) or Input.is_physical_key_pressed(KEY_UP):
		v.y -= 1
	if Input.is_physical_key_pressed(KEY_S) or Input.is_physical_key_pressed(KEY_DOWN):
		v.y += 1
	v += hud.joystick.value
	return v.limit_length(1.0)

func _move(delta: float) -> void:
	var v := Vector2.ZERO
	if modal == null and not hud.editing:
		v = _input_vector()
	if v != Vector2.ZERO:
		path.clear() # manual control cancels tap-to-move
	elif not path.is_empty():
		var goal: Vector2 = path[0]
		var d := goal - pos
		if d.length() < 0.12:
			path.pop_front()
			if path.is_empty() and path_target != "":
				target_id = path_target
				path_target = ""
				_try_attack()
		else:
			v = d.normalized()
	var speed := Game.move_speed() * 0.95
	if v != Vector2.ZERO:
		var step := v * speed * delta
		var np := pos + step
		if map.walkable_point(np):
			pos = np
		elif map.walkable_point(Vector2(np.x, pos.y)):
			pos.x = np.x
		elif map.walkable_point(Vector2(pos.x, np.y)):
			pos.y = np.y
		player.face_towards(v)
		player.moving = true
	else:
		player.moving = false
	player.position = pos * TILE
	var now := Time.get_ticks_msec() / 1000.0
	var changed := pos.distance_to(_last_sent) > 0.01
	if not _move_inflight and ((changed and now - _last_send_t >= 0.1) or now - _last_send_t > 20.0):
		_send_move(now)

func _send_move(now: float) -> void:
	_move_inflight = true
	_last_send_t = now
	var sent := pos
	_move_seq += 1
	var r := await Realtime.call_rpc("move", {"x": sent.x, "y": sent.y, "dir": player.dir, "anim": "walk" if player.moving else "idle", "seq": _move_seq})
	_move_inflight = false
	if not r.ok:
		return
	_last_sent = sent
	var sp := Vector2(float(r.data.position.x), float(r.data.position.y))
	if not r.data.accepted and str(r.data.position.zone) == zone:
		# the server disagreed (speed/collision): snap back to its position
		pos = sp
		_last_sent = sp
		path.clear()

# ---------------------------------------------------------------- combat

func _pick_target() -> MonsterNode:
	var rng := Game.attack_range() + 0.3
	if target_id != "" and monsters.has(target_id):
		var m: MonsterNode = monsters[target_id]
		if m.alive() and m.tile_pos().distance_to(pos) <= rng:
			return m
	var best: MonsterNode = null
	var best_d := rng
	for m in monsters.values():
		if not m.alive():
			continue
		var d: float = m.tile_pos().distance_to(pos)
		if d <= best_d:
			best_d = d
			best = m
	return best

func _set_target(id: String) -> void:
	if target_id != "" and monsters.has(target_id):
		monsters[target_id].targeted = false
		monsters[target_id].queue_redraw()
	target_id = id
	if id != "" and monsters.has(id):
		monsters[id].targeted = true
		monsters[id].queue_redraw()

func _try_attack() -> void:
	var now := Time.get_ticks_msec() / 1000.0
	if not ready_to_play or now < _next_attack_t or modal != null:
		return
	var m := _pick_target()
	if m == null:
		if now - _no_target_t > 1.5:
			_no_target_t = now
			Game.toast("No target in range", UiKit.MUTED)
		return
	_set_target(m.id)
	var cd := 0.7
	if Game.character.has("derived"):
		cd = float(Game.character.derived.get("cooldown", 0.7))
	_next_attack_t = now + cd
	player.face_towards(m.tile_pos() - pos)
	match Game.weapon_kind():
		"ranged":
			player.play_once("shoot")
		"magic":
			player.play_once("spellcast")
		_:
			player.play_once("slash")
	var r := await Realtime.call_rpc("attack", {"target": m.id})
	if not r.ok:
		var e := str(r.error)
		if e.find("cooldown") < 0:
			Game.toast(e.get_slice(": ", 1) if e.find(": ") >= 0 else e, UiKit.MUTED)
		return
	var d: Dictionary = r.data
	if is_instance_valid(m):
		var el := str(d.hit.get("element", ""))
		var dcol: Color = LpcSprite.ELEMENT_COLORS.get(el, Color.WHITE)
		if d.hit.crit:
			dcol = Color("ffd24a") if el == "" else dcol.lightened(0.3)
		FloatingText.spawn(map.entities, m.position, str(int(d.hit.damage)) + ("!" if d.hit.crit else ""), dcol, 18 if d.hit.crit else 14)
		if el != "":
			m.burst(dcol)
		if d.killed:
			m.die(Time.get_unix_time_from_system() * 1000.0 + 45000.0)
			FloatingText.spawn(map.entities, player.position + Vector2(0, -30), "+%d XP" % int(d.xp), Color("7ab8ff"), 14)
			_set_target("")
		else:
			m.hit(int(d.target_hp), int(d.target_max))
	if d.get("counter") is Dictionary:
		FloatingText.spawn(map.entities, player.position + Vector2(0, -20), "-%d" % int(d.counter.damage), UiKit.DANGER, 14)
		player.play_once("hurt")
		Telegram.haptic("light")
	Game.set_vitals(int(d.player_hp), int(d.player_max))

# ---------------------------------------------------------------- interaction

func _interact() -> void:
	if not ready_to_play or modal != null:
		return
	var near := map.nearest_interactive(pos)
	if near.is_empty():
		Game.toast("Nothing to use here", UiKit.MUTED)
		return
	var r := await Realtime.call_rpc("interact", {"x": near.x, "y": near.y})
	if not r.ok:
		var e := str(r.error)
		Game.toast(e.get_slice(": ", 1) if e.find(": ") >= 0 else e, UiKit.DANGER)
		return
	var d: Dictionary = r.data
	match str(d.get("action", "")):
		"dungeon_enter", "dungeon_descend":
			var v: Dictionary = d.dungeon
			Game.toast("%s - floor %d/%d" % [str(v.name), int(v.floor.floor) + 1, int(v.floor.floors)], UiKit.ACCENT)
			await _enter_zone(v.position, v.floor)
		"dungeon_leave":
			await _enter_zone(d.position)
		"chest":
			Game.toast("The chest opens!", Color("ffd24a"))
		"open_forge":
			_open(InventoryPanel.new())
		"shrine":
			Game.toast(str(d.text), Color("9fe0ff"))

# ---------------------------------------------------------------- realtime events

func _on_publication(channel: String, data: Dictionary) -> void:
	var t := str(data.get("t", ""))
	var me := str(Game.character.get("id", ""))
	if channel.begins_with("personal:"):
		_on_personal(t, data)
		return
	if channel.begins_with("news:"):
		if t == "world_first":
			Game.toast(str(data.text), Color("ffd27a"))
		return
	match t:
		"mv":
			var id := str(data.id)
			if id != me:
				_upsert_remote(id, Vector2(float(data.x), float(data.y)), str(data.get("d", "down")))
		"gone":
			var id := str(data.id)
			if remotes.has(id):
				remotes[id].node.queue_free()
				remotes.erase(id)
		"look":
			if remotes.has(str(data.id)):
				_load_remote_look(str(data.id))
		"hit":
			var id := str(data.id)
			if monsters.has(id) and str(data.get("by", "")) != me:
				monsters[id].hit(int(data.hp), int(data.max))
		"die":
			var id := str(data.id)
			if monsters.has(id):
				monsters[id].die(int(data.until))

func _on_personal(t: String, d: Dictionary) -> void:
	match t:
		"loot":
			if int(d.get("gold", 0)) > 0:
				Game.toast("+%d gold" % int(d.gold), Color("ffd24a"))
			var found = d.get("items")
			for it in (found if found is Array else []):
				Game.toast("Found: " + str(it.name), Sprites.rarity_color(str(it.rarity)))
			var lvls = d.get("item_levels")
			for lv in (lvls if lvls is Array else []):
				Game.toast("%s reached level %d" % [str(lv.item_name), int(lv.level)], Color("9fe0ff"))
			if d.get("auto_salvaged", false):
				Game.toast("Bag full: drops were salvaged", UiKit.MUTED)
			Game.refresh_inventory()
		"xp":
			Game.refresh_character()
		"level_up":
			Game.toast("LEVEL UP! Level %d (+%d points)" % [int(d.level), int(d.points)], UiKit.ACCENT)
			Telegram.haptic("success")
			Game.refresh_character()
		"awakened":
			Game.toast("Your class awakens: %s" % str(d["class"]).capitalize(), Color("b4a0ff"))
			Game.refresh_character()
		"teleport":
			await _enter_zone(d)
		"died":
			Game.toast("You were slain by %s" % str(d.by), UiKit.DANGER)
			Telegram.haptic("error")
		"dungeon_cleared":
			Game.toast("%s cleared!" % str(d.name), UiKit.ACCENT)

# ---------------------------------------------------------------- menus & input

func _open(m: Control) -> void:
	if modal:
		modal.queue_free()
	modal = m
	overlay.add_child(m)
	m.tree_exited.connect(func(): if modal == m: modal = null)

func _on_menu(action: String) -> void:
	match action:
		"inventory":
			_open(InventoryPanel.new())
		"character":
			_open(CharacterPanel.new())
		"history":
			_open(HistoryPanel.new())
		"layout":
			hud.start_editing()
		"exit":
			var now := Time.get_ticks_msec() / 1000.0
			if now - _exit_armed < 2.5:
				exit_to_menu.emit()
			else:
				_exit_armed = now
				Game.toast("Press Exit again to leave", UiKit.MUTED)

func _unhandled_input(e: InputEvent) -> void:
	if e is InputEventKey and e.pressed and not e.echo:
		match e.physical_keycode:
			KEY_E:
				_interact()
			KEY_I:
				_on_menu("inventory")
			KEY_C:
				_on_menu("character")
			KEY_ESCAPE:
				if modal:
					modal.queue_free()
	elif e is InputEventScreenTouch and e.pressed and modal == null and not hud.editing and ready_to_play:
		if hud.is_over_ui(e.position):
			return
		var world_px: Vector2 = get_viewport().get_canvas_transform().affine_inverse() * e.position
		_tap(world_px / TILE)

## Tap on a monster: target it and walk into range. Tap on ground: walk there.
func _tap(tile_pos: Vector2) -> void:
	var tapped := ""
	for m in monsters.values():
		if m.alive() and m.tile_pos().distance_to(tile_pos) < 0.9:
			tapped = m.id
	if tapped != "":
		_set_target(tapped)
		var mt: Vector2 = monsters[tapped].tile_pos()
		if mt.distance_to(pos) <= Game.attack_range():
			_try_attack()
			return
		_walk_to(Vector2i(floori(mt.x), floori(mt.y)), true)
		path_target = tapped
		return
	_walk_to(Vector2i(floori(tile_pos.x), floori(tile_pos.y)), false)

## Path over the locally known map (see Pathfinder: heap A* + smoothing).
func _walk_to(goal: Vector2i, adjacent: bool) -> bool:
	var start := Vector2i(floori(pos.x), floori(pos.y))
	var walk := func(t: Vector2i) -> bool: return map.walkable_point(Vector2(t) + Vector2(0.5, 0.5))
	var p := Pathfinder.find(start, goal, adjacent, walk)
	if p.is_empty() and not (adjacent and maxi(absi(start.x - goal.x), absi(start.y - goal.y)) <= 1):
		return false
	path = p
	return true

# ---------------------------------------------------------------- autotest

var _autotest_stage := "wait"
var _autotest_kill := ""

## Headless smoke test: run with `--autotest --dev-user=NAME`. Plays until one
## monster is killed, then exits with code 0 (or 1 on timeout).
func _autotest(now: float) -> void:
	if now - _autotest_start > 120.0:
		print("AUTOTEST FAIL at stage ", _autotest_stage)
		get_tree().quit(1)
		return
	match _autotest_stage:
		"wait":
			if ready_to_play and not monsters.is_empty() and map.chunks.size() >= 4:
				var best: MonsterNode = null
				for m in monsters.values():
					if m.alive() and str(m.spawn.rank) == "normal" and (best == null or m.tile_pos().distance_to(pos) < best.tile_pos().distance_to(pos)):
						best = m
				if best:
					_autotest_stage = "busy"
					await get_tree().create_timer(2.0).timeout
					await Cfg.shot("02_world")
					if Cfg.shot_dir != "":
						for el in ["fire", "lightning", "frost"]:
							player.set_fx(Game.character.appearance, {"element": el, "tier": 3, "rarity": "mythic"})
							player.play_once("slash")
							camera.zoom = Vector2(4, 4)
							await get_tree().create_timer(1.2).timeout
							await Cfg.shot("06_fx_" + el)
						_fit_camera()
						player.set_fx(Game.character.appearance, Game.character.get("fx"))
					print("AUTOTEST hunting ", best.id)
					_autotest_kill = best.id
					_tap(best.tile_pos())
					_autotest_stage = "hunt"
		"hunt":
			if path.is_empty():
				_attack_held = true
			if monsters.has(_autotest_kill) and not monsters[_autotest_kill].alive():
				_attack_held = false
				print("AUTOTEST killed ", _autotest_kill)
				_autotest_stage = "done"
				_on_menu("inventory")
		"done":
			_autotest_stage = "busy"
			await get_tree().create_timer(2.5).timeout
			await Cfg.shot("03_inventory")
			modal.queue_free()
			_on_menu("character")
			await get_tree().create_timer(1.5).timeout
			await Cfg.shot("04_hero")
			modal.queue_free()
			hud.start_editing()
			await get_tree().create_timer(0.5).timeout
			await Cfg.shot("05_layout_editor")
			hud.stop_editing(false)
			print("AUTOTEST PASS")
			get_tree().quit(0)
