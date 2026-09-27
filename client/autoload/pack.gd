extends Node
## The art pack rendered with Blender (tools/blender) and served by the
## main server at /api/v1/sprites/pack/: 3D-rendered UI icons and the
## pixel-art atlas of world props (trees, rocks, torches...). Every file has
## its content hash in its name, so the browser caches it forever and new
## art ships by updating the server only; the client needs no update.
##
## If the server is unreachable the game falls back to its built-in art
## (SVG icons, procedural props).

signal ready_changed

const BASE := "/api/v1/sprites/pack/"

var manifest := {}
var loaded := false
var props_texture: Texture2D
## GUI kit atlas (frames, buttons, slots, bars...) and its parts:
## name -> {rect: [x, y, w, h], margin: [l, t, r, b] or null}
var gui_texture: Texture2D
var gui_parts := {}
var gui_scale := 0.5
var failed := false
var _icons := {} # name -> Texture2D
var _loading := {} # name -> true

func _ready() -> void:
	load_manifest()

func load_manifest() -> void:
	var r := await Api.get_json(BASE + "manifest.json")
	if not r.ok or not r.data is Dictionary or int(r.data.get("format", 0)) != 1:
		push_warning("art pack unavailable: " + str(r.error))
		failed = true
		ready_changed.emit()
		return
	manifest = r.data
	var gui: Dictionary = manifest.get("gui", {})
	if gui.has("image"):
		var gt := await Sprites.fetch(BASE + str(gui.image))
		if not Sprites.is_placeholder(gt):
			var img := gt.get_image()
			img.generate_mipmaps()
			gui_texture = ImageTexture.create_from_image(img)
			gui_parts = gui.get("parts", {})
			gui_scale = float(gui.get("scale", 0.5))
	var props: Dictionary = manifest.get("props", {})
	if props.has("image"):
		var tex := await Sprites.fetch(BASE + str(props.image))
		if not Sprites.is_placeholder(tex):
			props_texture = tex
	loaded = true
	ready_changed.emit()

## Waits (up to `timeout` seconds) until the pack has loaded or failed, so
## the UI theme can be built from the GUI kit.
func wait_loaded(timeout: float) -> void:
	var end := Time.get_ticks_msec() + int(timeout * 1000)
	while not loaded and not failed and Time.get_ticks_msec() < end:
		await get_tree().process_frame

func has_part(part: String) -> bool:
	return gui_texture != null and gui_parts.has(part)

## A 9-slice stylebox of a GUI kit part, or null.
func nine(part: String, pad := [8, 8, 8, 8], scale := 0.0) -> NineBox:
	if not has_part(part):
		return null
	var p: Dictionary = gui_parts[part]
	var rc: Array = p.rect
	var mg = p.get("margin")
	return NineBox.new(gui_texture, Rect2(rc[0], rc[1], rc[2], rc[3]), mg if mg is Array else [0, 0, 0, 0], scale if scale > 0 else gui_scale, pad)

## A whole part as a texture (rings, close button, separators), or null.
func part_texture(part: String) -> Texture2D:
	if not has_part(part):
		return null
	var rc: Array = gui_parts[part].rect
	var at := AtlasTexture.new()
	at.atlas = gui_texture
	at.region = Rect2(rc[0], rc[1], rc[2], rc[3])
	return at

func has_icon(icon: String) -> bool:
	return manifest.get("icons", {}).has(icon)

## The rendered icon, or null when the pack has none (or it failed).
## Loaded once, with mipmaps so it stays smooth at small sizes.
func icon(icon: String) -> Texture2D:
	if not loaded:
		await ready_changed
	if _icons.has(icon):
		return _icons[icon]
	if not has_icon(icon):
		return null
	if _loading.has(icon):
		while _loading.has(icon):
			await get_tree().process_frame
		return _icons.get(icon)
	_loading[icon] = true
	var tex := await Sprites.fetch(BASE + str(manifest.icons[icon]))
	var out: Texture2D = null
	if not Sprites.is_placeholder(tex):
		var img := tex.get_image()
		img.generate_mipmaps()
		out = ImageTexture.create_from_image(img)
	_icons[icon] = out
	_loading.erase(icon)
	return out

## Prop atlas layout: object id (String) -> [{at: [x, y], size: [w, h], base: [bx, by]}].
func prop_layout() -> Dictionary:
	return manifest.get("props", {}).get("objects", {}) if props_texture else {}
