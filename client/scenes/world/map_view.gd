class_name MapView
extends Node2D
## Renders terrain from server chunk data (overworld) or a dungeon floor,
## and answers local collision queries with the same rules as the server.

const TILE := 32
const CHUNK := 32
const GROUND_VARIANTS := 4
const GROUND_COUNT := 16
const OBJECT_COUNT := 18
const OBJECT_ROW := 16
const WALL := 12
# ground ids that cannot be walked on: deep water, water, dungeon wall, lava
const BLOCKED_GROUND := [0, 1, 12, 14]
# object ids that block: tree, pine, rock, cactus, dead tree, boulder, chest, torch, anvil, shrine
const BLOCKING := [1, 2, 3, 6, 7, 8, 13, 14, 15, 16]
const INTERACTIVE := [10, 11, 12, 13, 15, 16]
const GROUND_COLORS := [
	Color("183e7a"), Color("2c68b6"), Color("dec892"), Color("58a048"), Color("347238"),
	Color("8e704a"), Color("e8eef4"), Color("506444"), Color("7a7670"), Color("4a4242"),
	Color("bca274"), Color("5e5854"), Color("2e2a36"), Color("b6def0"), Color("de5418"), Color("4a8ecc"),
]

var ground: TileMapLayer
var objects: TileMapLayer
var entities: Node2D
var chunks := {} # Vector2i -> {ground: PackedByteArray, objects: PackedByteArray}
var floor_data := {} # dungeon floor, when inside one
var in_dungeon := false

func setup(tileset_texture: Texture2D) -> void:
	var ts := TileSet.new()
	ts.tile_size = Vector2i(TILE, TILE)
	var src := TileSetAtlasSource.new()
	src.texture = tileset_texture
	src.texture_region_size = Vector2i(TILE, TILE)
	for g in GROUND_COUNT:
		for v in GROUND_VARIANTS:
			src.create_tile(Vector2i(v, g))
	for o in range(1, OBJECT_COUNT):
		src.create_tile(Vector2i(o, OBJECT_ROW))
		# sort props by their base so characters walk behind/in front of them
		src.get_tile_data(Vector2i(o, OBJECT_ROW), 0).y_sort_origin = 12
	ts.add_source(src, 0)
	ground = TileMapLayer.new()
	ground.tile_set = ts
	ground.z_index = -10
	add_child(ground)
	var sorter := Node2D.new()
	sorter.y_sort_enabled = true
	add_child(sorter)
	objects = TileMapLayer.new()
	objects.tile_set = ts
	objects.y_sort_enabled = true
	sorter.add_child(objects)
	entities = sorter

func clear() -> void:
	ground.clear()
	objects.clear()
	chunks.clear()
	floor_data = {}
	in_dungeon = false

static func decode_bytes(v) -> PackedByteArray:
	if v is String:
		return Marshalls.base64_to_raw(v)
	return PackedByteArray()

static func variant_of(x: int, y: int) -> int:
	var h := (x * 73856093) ^ (y * 19349663)
	return absi(h) % GROUND_VARIANTS

func _paint(x: int, y: int, g: int, o: int) -> void:
	var cell := Vector2i(x, y)
	ground.set_cell(cell, 0, Vector2i(variant_of(x, y), clampi(g, 0, GROUND_COUNT - 1)))
	if o > 0 and o < OBJECT_COUNT:
		objects.set_cell(cell, 0, Vector2i(o, OBJECT_ROW))
	else:
		objects.erase_cell(cell)

func add_chunk(data: Dictionary) -> void:
	var t: Dictionary = data.terrain
	var key := Vector2i(int(t.cx), int(t.cy))
	var g := decode_bytes(t.ground)
	var o := decode_bytes(t.objects)
	chunks[key] = {"ground": g, "objects": o}
	var n: int = int(t.size)
	for i in g.size():
		_paint(key.x * n + i % n, key.y * n + i / n, g[i], o[i])

func remove_chunk(key: Vector2i) -> void:
	if not chunks.has(key):
		return
	chunks.erase(key)
	for y in CHUNK:
		for x in CHUNK:
			var cell := Vector2i(key.x * CHUNK + x, key.y * CHUNK + y)
			ground.erase_cell(cell)
			objects.erase_cell(cell)

func set_floor(f: Dictionary) -> void:
	clear()
	in_dungeon = true
	floor_data = {"w": int(f.w), "h": int(f.h), "ground": decode_bytes(f.ground), "objects": decode_bytes(f.objects)}
	var w: int = floor_data.w
	for y in range(-2, int(f.h) + 2):
		for x in range(-2, w + 2):
			var gi := WALL
			var oi := 0
			if x >= 0 and y >= 0 and x < w and y < int(f.h):
				gi = floor_data.ground[y * w + x]
				oi = floor_data.objects[y * w + x]
			_paint(x, y, gi, oi)

## Returns [ground, object] for a tile, or [-1, 0] when unknown.
func tile(x: int, y: int) -> Array:
	if in_dungeon:
		var w: int = floor_data.w
		if x < 0 or y < 0 or x >= w or y >= int(floor_data.h):
			return [WALL, 0]
		return [floor_data.ground[y * w + x], floor_data.objects[y * w + x]]
	var key := Vector2i(floori(float(x) / CHUNK), floori(float(y) / CHUNK))
	if not chunks.has(key):
		return [-1, 0]
	var c: Dictionary = chunks[key]
	var i := (y - key.y * CHUNK) * CHUNK + (x - key.x * CHUNK)
	return [c.ground[i], c.objects[i]]

func walkable_point(p: Vector2) -> bool:
	if p.x < 0 or p.y < 0:
		return false
	var t := tile(floori(p.x), floori(p.y))
	return t[0] >= 0 and not (t[0] in BLOCKED_GROUND) and not (t[1] in BLOCKING)

func set_object(x: int, y: int, o: int) -> void:
	if in_dungeon:
		floor_data.objects[y * int(floor_data.w) + x] = o
	if o > 0:
		objects.set_cell(Vector2i(x, y), 0, Vector2i(o, OBJECT_ROW))
	else:
		objects.erase_cell(Vector2i(x, y))

## Nearest interactive object within `reach` tiles: {x, y, object} or {}.
func nearest_interactive(p: Vector2, reach := 1.9) -> Dictionary:
	var best := {}
	var best_d := reach
	for dy in range(-2, 3):
		for dx in range(-2, 3):
			var x := floori(p.x) + dx
			var y := floori(p.y) + dy
			var t := tile(x, y)
			if t[1] in INTERACTIVE:
				var d := p.distance_to(Vector2(x + 0.5, y + 0.5))
				if d <= best_d:
					best_d = d
					best = {"x": x, "y": y, "object": t[1]}
	return best

## Minimap image of the known area around `center` (tile coords).
func minimap_image(center: Vector2, radius := 48) -> Image:
	var size := radius * 2
	var img := Image.create(size, size, false, Image.FORMAT_RGBA8)
	img.fill(Color(0, 0, 0, 0.6))
	var cx := floori(center.x)
	var cy := floori(center.y)
	for y in size:
		for x in size:
			var t := tile(cx - radius + x, cy - radius + y)
			if t[0] < 0:
				continue
			var col: Color = GROUND_COLORS[clampi(t[0], 0, GROUND_COLORS.size() - 1)]
			if t[1] in [1, 2, 8]:
				col = col.darkened(0.35)
			elif t[1] == 10 or t[1] == 11:
				col = Color("ff40ff")
			elif t[1] == 16:
				col = Color("ffe060")
			img.set_pixel(x, y, col)
	return img
