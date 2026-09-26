package sprite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"sort"
	"strconv"
	"strings"

	"github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

// Frame is the size of one LPC frame.
const Frame = 64

// AnimLayout is where an animation lives in the universal sheet.
type AnimLayout struct {
	Name   string `json:"name"`
	Row    int    `json:"row"`
	Rows   int    `json:"rows"`
	Frames int    `json:"frames"`
}

// Layout is the classic LPC universal sheet (832x1344). Directions within a
// 4-row animation are ordered up, left, down, right.
var Layout = []AnimLayout{
	{"spellcast", 0, 4, 7},
	{"thrust", 4, 4, 8},
	{"walk", 8, 4, 9},
	{"slash", 12, 4, 6},
	{"shoot", 16, 4, 13},
	{"hurt", 20, 1, 6},
}

const SheetW, SheetH = 13 * Frame, 21 * Frame

// Composer renders recipes into sprite sheets.
type Composer struct {
	Cat    *Catalog
	Assets *Assets
}

// Normalize validates a recipe against the catalog, dropping parts that
// don't exist or don't fit the body type, and guaranteeing a body layer.
func (c *Composer) Normalize(r contracts.Recipe) (contracts.Recipe, []string) {
	var dropped []string
	if !contains(c.Cat.BodyTypes, r.BodyType) {
		r.BodyType = "male"
	}
	out := contracts.Recipe{BodyType: r.BodyType}
	hasBody := false
	seen := map[string]bool{}
	for _, l := range r.Layers {
		it, ok := c.Cat.Items[l.Item]
		if !ok || !it.Supports(r.BodyType) || seen[l.Item] {
			dropped = append(dropped, l.Item)
			continue
		}
		seen[l.Item] = true
		if len(it.Variants) > 0 && !contains(it.Variants, l.Variant) {
			l.Variant = it.Variants[0]
		}
		if len(it.Variants) == 0 {
			l.Variant = ""
		}
		if len(l.Colors) > len(it.Recolors) {
			l.Colors = l.Colors[:len(it.Recolors)]
		}
		if it.TypeName == "body" {
			hasBody = true
		}
		out.Layers = append(out.Layers, l)
	}
	if !hasBody {
		out.Layers = append([]contracts.Layer{{Item: "body", Colors: []string{"light"}}}, out.Layers...)
	}
	return out, dropped
}

// Hash is the cache key of a (normalised) recipe.
func (c *Composer) Hash(r contracts.Recipe) string {
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(append(raw, []byte(c.Cat.Source.Commit)...))
	return hex.EncodeToString(sum[:12])
}

type drawOp struct {
	z     int
	order int
	item  *Item
	layer ItemLayer
	sel   contracts.Layer
}

// Render composes the full universal sheet for a recipe.
func (c *Composer) Render(ctx context.Context, r contracts.Recipe) (*image.NRGBA, error) {
	r, _ = c.Normalize(r)
	bodyColor := ""
	for _, l := range r.Layers {
		if it := c.Cat.Items[l.Item]; it != nil && it.TypeName == "body" && len(l.Colors) > 0 {
			bodyColor = l.Colors[0]
		}
	}
	var ops []drawOp
	for i, l := range r.Layers {
		it := c.Cat.Items[l.Item]
		for _, layer := range it.Layers {
			if _, ok := layer.Paths[r.BodyType]; ok {
				ops = append(ops, drawOp{z: layer.Z, order: i, item: it, layer: layer, sel: l})
			}
		}
	}
	sort.SliceStable(ops, func(a, b int) bool { return ops[a].z < ops[b].z })
	sheet := image.NewNRGBA(image.Rect(0, 0, SheetW, SheetH))
	drawn := 0
	for _, op := range ops {
		mappings := c.mappings(op.item, op.sel, bodyColor)
		dir := op.layer.Paths[r.BodyType]
		for _, anim := range Layout {
			if !op.item.HasAnimation(anim.Name) {
				continue
			}
			rel := dir + anim.Name + ".png"
			if op.sel.Variant != "" {
				rel = dir + anim.Name + "/" + strings.ReplaceAll(op.sel.Variant, " ", "_") + ".png"
			}
			img, err := c.Assets.Image(ctx, rel)
			if errors.Is(err, ErrMissing) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if len(mappings) > 0 {
				img = recolor(img, mappings)
			}
			dst := image.Rect(0, anim.Row*Frame, SheetW, (anim.Row+anim.Rows)*Frame)
			draw.Draw(sheet, dst, img, image.Point{}, draw.Over)
			drawn++
		}
	}
	if drawn == 0 {
		return nil, fmt.Errorf("no layer images could be loaded (is the asset store reachable?)")
	}
	return sheet, nil
}

// RenderPNG renders and encodes.
func (c *Composer) RenderPNG(ctx context.Context, r contracts.Recipe) ([]byte, error) {
	img, err := c.Render(ctx, r)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type rgb struct{ r, g, b uint8 }

type pair struct{ src, dst rgb }

func (c *Composer) mappings(it *Item, sel contracts.Layer, bodyColor string) []pair {
	var out []pair
	for i, rc := range it.Recolors {
		target := ""
		if i < len(sel.Colors) {
			target = sel.Colors[i]
		}
		if target == "" && i == 0 && it.MatchBodyColor {
			target = bodyColor
		}
		if target == "" || target == "source" {
			continue
		}
		src, ok := c.Cat.sourcePalette(rc)
		if !ok {
			continue
		}
		dst, ok := c.Cat.resolvePalette(rc.Material, target)
		if !ok {
			continue
		}
		for k := 0; k < len(src) && k < len(dst); k++ {
			s, ok1 := parseHex(src[k])
			d, ok2 := parseHex(dst[k])
			if ok1 && ok2 {
				out = append(out, pair{s, d})
			}
		}
	}
	return out
}

// recolor maps palette colours (±1 per channel tolerance, like the LPC tool).
func recolor(src *image.NRGBA, pairs []pair) *image.NRGBA {
	out := image.NewNRGBA(src.Rect)
	copy(out.Pix, src.Pix)
	cache := map[rgb]rgb{}
	for i := 0; i+3 < len(out.Pix); i += 4 {
		if out.Pix[i+3] == 0 {
			continue
		}
		px := rgb{out.Pix[i], out.Pix[i+1], out.Pix[i+2]}
		to, ok := cache[px]
		if !ok {
			to = px
			for _, p := range pairs {
				if near(px.r, p.src.r) && near(px.g, p.src.g) && near(px.b, p.src.b) {
					to = p.dst
					break
				}
			}
			cache[px] = to
		}
		out.Pix[i], out.Pix[i+1], out.Pix[i+2] = to.r, to.g, to.b
	}
	return out
}

func near(a, b uint8) bool { return int(a)-int(b) <= 1 && int(b)-int(a) <= 1 }

func parseHex(s string) (rgb, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) < 6 {
		return rgb{}, false
	}
	v, err := strconv.ParseUint(s[:6], 16, 32)
	if err != nil {
		return rgb{}, false
	}
	return rgb{uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Credits lists the attribution for every item used in a recipe.
func (c *Composer) Credits(r contracts.Recipe) []Credit {
	r, _ = c.Normalize(r)
	var out []Credit
	seen := map[string]bool{}
	for _, l := range r.Layers {
		it := c.Cat.Items[l.Item]
		for _, cr := range it.Credits {
			if !seen[cr.File] {
				seen[cr.File] = true
				out = append(out, cr)
			}
		}
	}
	return out
}
