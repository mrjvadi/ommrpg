class_name NineBox
extends StyleBox
## 9-slice box from the GUI atlas rendered with Blender (see Pack). The
## atlas is drawn at 2x, so the corners are drawn at `scale` (0.5) of their
## texture size: sharp on phones, while only the middle strips stretch.

var texture: Texture2D
var region := Rect2() # the part's rect in the atlas (texture pixels)
var margin := [0, 0, 0, 0] # left, top, right, bottom (texture pixels)
var scale := 0.5
var modulate := Color.WHITE
var draw_center := true
## Draw this many pixels inside the given rect (a bar's fill inside its frame).
var inset := 0.0
## Repeat the edge strips instead of stretching them (engraved frames whose
## pattern must keep its spacing). The middle strip of such a part is a whole
## number of pattern periods, so the copies join seamlessly.
var tile := false

func _init(tex: Texture2D = null, rect := Rect2(), margins := [0, 0, 0, 0], s := 0.5, pad := [8, 8, 8, 8]) -> void:
	texture = tex
	region = rect
	margin = margins
	scale = s
	content_margin_left = pad[0]
	content_margin_top = pad[1]
	content_margin_right = pad[2]
	content_margin_bottom = pad[3]

func copy() -> NineBox:
	var b := NineBox.new(texture, region, margin, scale, [content_margin_left, content_margin_top, content_margin_right, content_margin_bottom])
	b.modulate = modulate
	b.draw_center = draw_center
	b.inset = inset
	b.tile = tile
	return b

func _draw(ci: RID, rect: Rect2) -> void:
	if texture == null:
		return
	if inset > 0:
		rect = rect.grow(-inset)
		if rect.size.x <= 0 or rect.size.y <= 0:
			return
	var rid := texture.get_rid()
	var l := float(margin[0])
	var t := float(margin[1])
	var r := float(margin[2])
	var b := float(margin[3])
	# destination margins, shrunk proportionally if the box is too small
	var dl := l * scale
	var dr := r * scale
	var dt := t * scale
	var db := b * scale
	if dl + dr > rect.size.x and dl + dr > 0:
		var k := rect.size.x / (dl + dr)
		dl *= k
		dr *= k
	if dt + db > rect.size.y and dt + db > 0:
		var k := rect.size.y / (dt + db)
		dt *= k
		db *= k
	var sx := [region.position.x, region.position.x + l, region.end.x - r, region.end.x]
	var sy := [region.position.y, region.position.y + t, region.end.y - b, region.end.y]
	var dx := [rect.position.x, rect.position.x + dl, rect.end.x - dr, rect.end.x]
	var dy := [rect.position.y, rect.position.y + dt, rect.end.y - db, rect.end.y]
	for j in 3:
		for i in 3:
			if i == 1 and j == 1 and not draw_center:
				continue
			var dst := Rect2(dx[i], dy[j], dx[i + 1] - dx[i], dy[j + 1] - dy[j])
			var src := Rect2(sx[i], sy[j], sx[i + 1] - sx[i], sy[j + 1] - sy[j])
			if dst.size.x <= 0.01 or dst.size.y <= 0.01 or src.size.x <= 0 or src.size.y <= 0:
				continue
			if tile and (i == 1) != (j == 1):
				_tiled(ci, rid, dst, src, i == 1)
			else:
				RenderingServer.canvas_item_add_texture_rect_region(ci, dst, rid, src, modulate, false, true)

## Repeats `src` along x (or y) at the box scale, cropping the last copy.
func _tiled(ci: RID, rid: RID, dst: Rect2, src: Rect2, along_x: bool) -> void:
	var step := (src.size.x if along_x else src.size.y) * scale
	var total := dst.size.x if along_x else dst.size.y
	if step < 1.0:
		return
	# centre the pattern so both ends are cut the same way
	var n := ceili(total / step)
	var start := (total - n * step) / 2.0
	for k in n:
		var a := maxf(0.0, start + k * step)
		var b := minf(total, start + (k + 1) * step)
		if b - a <= 0.01:
			continue
		var f0 := (a - (start + k * step)) / step
		var f1 := (b - (start + k * step)) / step
		if along_x:
			RenderingServer.canvas_item_add_texture_rect_region(ci, Rect2(dst.position.x + a, dst.position.y, b - a, dst.size.y), rid,
				Rect2(src.position.x + src.size.x * f0, src.position.y, src.size.x * (f1 - f0), src.size.y), modulate, false, true)
		else:
			RenderingServer.canvas_item_add_texture_rect_region(ci, Rect2(dst.position.x, dst.position.y + a, dst.size.x, b - a), rid,
				Rect2(src.position.x, src.position.y + src.size.y * f0, src.size.x, src.size.y * (f1 - f0)), modulate, false, true)
