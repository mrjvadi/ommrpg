extends Node
## Loads and caches textures produced by sprite-service: LPC character
## sheets (from appearance recipes), procedural creatures, item icons and the
## world tileset. Concurrent requests for the same url share one download.

signal loaded(url: String, texture: Texture2D)

const SHEET_FRAME := 64
## Classic LPC universal sheet: name -> [first row, rows, frames]. Rows of a
## 4-row animation face up, left, down, right.
const ANIMS := {
	"spellcast": [0, 4, 7],
	"thrust": [4, 4, 8],
	"walk": [8, 4, 9],
	"slash": [12, 4, 6],
	"shoot": [16, 4, 13],
	"hurt": [20, 1, 6],
}
const DIRS := {"up": 0, "left": 1, "down": 2, "right": 3}

var _cache := {} # url -> Texture2D
var _inflight := {} # url -> true
var _placeholder: Texture2D

func _ready() -> void:
	var img := Image.create(32, 32, false, Image.FORMAT_RGBA8)
	img.fill(Color(1, 0, 1, 0.0))
	_placeholder = ImageTexture.create_from_image(img)

## Returns the texture if cached; otherwise starts loading and returns null.
## Listen to `loaded` or use `fetch` to wait.
func peek(url: String) -> Texture2D:
	if _cache.has(url):
		return _cache[url]
	_start(url)
	return null

func fetch(url: String) -> Texture2D:
	if _cache.has(url):
		return _cache[url]
	_start(url)
	while true:
		var args: Array = await loaded
		if args[0] == url:
			return args[1]
	return null

func _start(url: String) -> void:
	if _inflight.has(url):
		return
	_inflight[url] = true
	var http := HTTPRequest.new()
	http.timeout = 30.0
	add_child(http)
	http.request_completed.connect(func(result: int, code: int, _h, body: PackedByteArray):
		http.queue_free()
		_inflight.erase(url)
		var tex: Texture2D = _placeholder
		if result == HTTPRequest.RESULT_SUCCESS and code == 200:
			var img := Image.new()
			if img.load_png_from_buffer(body) == OK:
				tex = ImageTexture.create_from_image(img)
				_cache[url] = tex
		else:
			push_warning("sprite %s failed: %d/%d" % [url, result, code])
		loaded.emit(url, tex)
	)
	var err := http.request(Cfg.url(url))
	if err != OK:
		http.queue_free()
		_inflight.erase(url)
		loaded.emit.call_deferred(url, _placeholder)

# ---------------------------------------------------------------- urls

## base64url(JSON) without padding, as expected by the sprite service.
static func encode_recipe(recipe: Dictionary) -> String:
	var b64 := Marshalls.utf8_to_base64(JSON.stringify(recipe))
	return b64.replace("+", "-").replace("/", "_").replace("=", "")

static func character_url(recipe: Dictionary) -> String:
	return "/api/v1/sprites/character.png?r=" + encode_recipe(recipe)

static func creature_url(species: Dictionary) -> String:
	return "/api/v1/sprites/creature.png?seed=%s&family=%s&hue=%.4f&hue2=%.4f&big=%d" % [
		str(species.get("sprite_seed", "0")), str(species.get("family", "slime")),
		float(species.get("hue", 0.0)), float(species.get("hue2", 0.0)), 1 if species.get("big", false) else 0]

static func icon_url(item: Dictionary) -> String:
	return "/api/v1/sprites/icon.png?seed=%s&shape=%s&hue=%.4f&rarity=%d" % [
		str(item.get("icon_seed", "0")), str(item.get("icon", "ring")), float(item.get("hue", 0.0)),
		rarity_index(str(item.get("rarity", "common")))]

const RARITIES := ["common", "uncommon", "rare", "epic", "legendary", "mythic"]
const RARITY_COLORS := [Color("c8c8c8"), Color("5adc5a"), Color("468cff"), Color("be5aff"), Color("ffa028"), Color("ff466e")]

static func rarity_index(r: String) -> int:
	return max(0, RARITIES.find(r))

static func rarity_color(r: String) -> Color:
	return RARITY_COLORS[rarity_index(r)]
