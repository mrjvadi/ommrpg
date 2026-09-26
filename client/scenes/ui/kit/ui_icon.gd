class_name UiIcon
extends TextureRect
## A UI icon. When the server's art pack has a 3D icon rendered with
## Blender (see Pack) it is used as is; otherwise, and while it downloads,
## the built-in game-icons.net silhouette (assets/icons/<name>.svg) is shown,
## recoloured with a gradient, outline and shadow (ui_icon.gdshader).
## Faded placeholders ("muted") always use the silhouette.

const SHADER := preload("res://assets/shaders/ui_icon.gdshader")
const PALETTES := {
	"gold": [Color("fff1b0"), Color("e0902a")],
	"silver": [Color("ffffff"), Color("a9b4c2")],
	"red": [Color("ff9a8a"), Color("c0281e")],
	"blue": [Color("bfe6ff"), Color("2f7fe0")],
	"green": [Color("c8ffb0"), Color("3aa84a")],
	"purple": [Color("f0c8ff"), Color("8a3fd0")],
	"ton": [Color("c8ecff"), Color("1f8fe8")],
	"bronze": [Color("f5d2a0"), Color("9a5a2a")],
	"muted": [Color("8f877f"), Color("5a524c")],
}

static var _textures := {}
var _name := ""
var _palette := "gold"
var _shader_mat: ShaderMaterial

static func tex(icon: String) -> Texture2D:
	if not _textures.has(icon):
		var path := "res://assets/icons/%s.svg" % icon
		_textures[icon] = load(path) if ResourceLoader.exists(path) else null
	return _textures[icon]

func _init(icon := "", size_px := 32.0, palette := "gold") -> void:
	expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	texture_filter = CanvasItem.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(size_px, size_px)
	_shader_mat = ShaderMaterial.new()
	_shader_mat.shader = SHADER
	material = _shader_mat
	_palette = palette
	set_palette(palette)
	set_icon(icon)

func set_icon(icon: String) -> void:
	_name = icon
	texture = UiIcon.tex(icon) if icon != "" else null
	material = _shader_mat
	if icon != "" and _palette != "muted":
		_use_pack(icon)

func _use_pack(icon: String) -> void:
	var t: Texture2D = await Pack.icon(icon)
	if t != null and is_instance_valid(self) and _name == icon and _palette != "muted":
		texture = t
		material = null

func set_palette(palette: String) -> void:
	_palette = palette
	var p: Array = PALETTES.get(palette, PALETTES.gold)
	set_colors(p[0], p[1])

func set_colors(top: Color, bottom: Color) -> void:
	_shader_mat.set_shader_parameter("top_color", top)
	_shader_mat.set_shader_parameter("bottom_color", bottom)
	_shader_mat.set_shader_parameter("outline_px", clampf(custom_minimum_size.x / 14.0, 1.5, 6.0) * 4.0)
