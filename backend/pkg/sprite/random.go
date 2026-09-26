package sprite

import (
	"sort"
	"strings"

	"github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

var (
	naturalSkin  = []string{"light", "amber", "olive", "taupe", "bronze", "brown", "black"}
	fantasySkin  = []string{"green", "blue", "lavender", "pale_green", "zombie", "bright_green"}
	naturalHair  = []string{"black", "dark_brown", "chestnut", "light_brown", "blonde", "ginger", "gray", "white", "sandy", "ash", "platinum", "strawberry", "orange", "carrot", "gold"}
	clothPalette = []string{"brown", "leather", "walnut", "tan", "maroon", "red", "blue", "navy", "teal", "forest", "green", "gray", "black", "charcoal", "white", "purple", "bluegray", "slate", "sky", "rose", "orange"}
	eyeColors    = []string{"blue", "brown", "green", "gray", "orange", "purple", "red", "yellow"}
)

// pool returns sorted item ids matching a predicate (sorted => deterministic).
func (c *Catalog) pool(pred func(*Item) bool) []string {
	var out []string
	for id, it := range c.Items {
		if pred(it) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func (c *Catalog) validColor(material, name string) bool {
	_, ok := c.resolvePalette(material, name)
	return ok
}

func (c *Catalog) pickColor(r *seed.Rand, it *Item, idx int, prefer []string) string {
	if idx >= len(it.Recolors) {
		return ""
	}
	mat := it.Recolors[idx].Material
	var ok []string
	for _, p := range prefer {
		if c.validColor(mat, p) {
			ok = append(ok, p)
		}
	}
	if len(ok) == 0 {
		ok = c.ColorOptions(it, idx)
	}
	if len(ok) == 0 {
		return ""
	}
	return seed.Pick(r, ok)
}

func (c *Catalog) wearable(bodyType, typeName, categoryPrefix string) []string {
	return c.pool(func(it *Item) bool {
		return it.TypeName == typeName && strings.HasPrefix(it.Category, categoryPrefix) &&
			it.Supports(bodyType) && it.HasAnimation("walk")
	})
}

func (c *Catalog) layerFor(r *seed.Rand, id string, prefer ...[]string) contracts.Layer {
	it := c.Items[id]
	l := contracts.Layer{Item: id}
	if len(it.Variants) > 0 {
		l.Variant = seed.Pick(r, it.Variants)
	}
	for i := range it.Recolors {
		var p []string
		if i < len(prefer) {
			p = prefer[i]
		}
		l.Colors = append(l.Colors, c.pickColor(r, it, i, p))
	}
	return l
}

// RandomRecipe deterministically dresses a character from a seed. Used for
// new player characters (the "randomise" button) and for generated NPCs.
func (c *Catalog) RandomRecipe(s uint64, bodyType string) contracts.Recipe {
	r := seed.New(seed.Derive(s, "appearance"))
	if !contains(c.BodyTypes, bodyType) || bodyType == "" {
		bodyType = seed.Pick(r, []string{"male", "female"})
	}
	rec := contracts.Recipe{BodyType: bodyType}
	skin := seed.Pick(r, naturalSkin)
	if r.Chance(0.06) {
		skin = seed.Pick(r, fantasySkin)
	}
	if !c.validColor("body", skin) {
		skin = "light"
	}
	rec.Layers = append(rec.Layers, contracts.Layer{Item: "body", Colors: []string{skin}})

	head := "heads_human_male"
	if bodyType == "female" || bodyType == "pregnant" {
		head = "heads_human_female"
	}
	if hi := c.Items[head]; hi != nil {
		l := contracts.Layer{Item: head, Colors: []string{skin}}
		if len(hi.Recolors) > 1 {
			l.Colors = append(l.Colors, c.pickColor(r, hi, 1, eyeColors))
		}
		rec.Layers = append(rec.Layers, l)
	}
	hairColor := seed.Pick(r, naturalHair)
	if hairs := c.wearable(bodyType, "hair", "hair/"); len(hairs) > 0 && !(bodyType == "male" && r.Chance(0.08)) {
		l := c.layerFor(r, seed.Pick(r, hairs), []string{hairColor})
		rec.Layers = append(rec.Layers, l)
	}
	if bodyType == "male" && r.Chance(0.25) {
		if beards := c.wearable(bodyType, "beard", "hair/beards"); len(beards) > 0 {
			rec.Layers = append(rec.Layers, c.layerFor(r, seed.Pick(r, beards), []string{hairColor}))
		}
	}
	if shirts := c.wearable(bodyType, "clothes", "torso/shirts"); len(shirts) > 0 {
		rec.Layers = append(rec.Layers, c.layerFor(r, seed.Pick(r, shirts), clothPalette))
	}
	legPrefix := "legs/pants"
	if bodyType == "female" && r.Chance(0.35) {
		legPrefix = "legs/skirts"
	}
	legs := c.wearable(bodyType, "legs", legPrefix)
	if len(legs) == 0 {
		legs = c.wearable(bodyType, "legs", "legs/pants")
	}
	if len(legs) > 0 {
		rec.Layers = append(rec.Layers, c.layerFor(r, seed.Pick(r, legs), clothPalette))
	}
	if shoes := c.wearable(bodyType, "shoes", "feet/"); len(shoes) > 0 {
		rec.Layers = append(rec.Layers, c.layerFor(r, seed.Pick(r, shoes), []string{"brown", "leather", "black", "walnut", "tan"}))
	}
	return rec
}
