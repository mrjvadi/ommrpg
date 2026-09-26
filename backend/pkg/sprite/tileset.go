package sprite

import (
	"image"
	"image/color"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

const (
	TileSize       = 32
	GroundVariants = 4
)

// TilesetMeta tells clients how to index the atlas.
type TilesetMeta struct {
	TileSize       int `json:"tile_size"`
	GroundVariants int `json:"ground_variants"`
	GroundRows     int `json:"ground_rows"` // ground id g, variant v -> (v, g)
	ObjectRow      int `json:"object_row"`  // object id o -> (o, object_row)
	Columns        int `json:"columns"`
}

var groundBase = map[world.Ground]color.NRGBA{
	world.GDeepWater: {24, 62, 122, 255}, world.GWater: {44, 104, 182, 255},
	world.GSand: {222, 202, 146, 255}, world.GGrass: {88, 160, 72, 255},
	world.GForest: {52, 114, 56, 255}, world.GDirt: {142, 112, 74, 255},
	world.GSnow: {232, 238, 244, 255}, world.GSwamp: {80, 100, 68, 255},
	world.GRock: {122, 118, 112, 255}, world.GAsh: {74, 66, 66, 255},
	world.GPath: {188, 162, 116, 255}, world.GDungeonFloor: {94, 88, 84, 255},
	world.GDungeonWall: {50, 46, 56, 255}, world.GIce: {182, 222, 240, 255},
	world.GLava: {222, 84, 24, 255},
}

// TilesetInfo describes the atlas layout without rendering it.
func TilesetInfo() TilesetMeta {
	cols := int(world.ObjectCount)
	if cols < GroundVariants {
		cols = GroundVariants
	}
	return TilesetMeta{TileSize: TileSize, GroundVariants: GroundVariants, GroundRows: int(world.GroundCount), ObjectRow: int(world.GroundCount), Columns: cols}
}

// Tileset renders the ground + object atlas used by clients.
func Tileset(s uint64) (*image.NRGBA, TilesetMeta) {
	meta := TilesetInfo()
	cols := meta.Columns
	atlas := newCanvas(cols*TileSize, (int(world.GroundCount)+1)*TileSize)
	for g := world.Ground(0); g < world.GroundCount; g++ {
		for v := 0; v < GroundVariants; v++ {
			t := groundTile(seed.Derive(s, "ground", int(g), v), g)
			atlas.blit(t, v*TileSize, int(g)*TileSize, 1)
		}
	}
	for o := world.Object(1); o < world.ObjectCount; o++ {
		t := objectTile(seed.Derive(s, "object", int(o)), o)
		atlas.blit(t, int(o)*TileSize, meta.ObjectRow*TileSize, 1)
	}
	return atlas.img, meta
}

func groundTile(s uint64, g world.Ground) *image.NRGBA {
	r := seed.New(s)
	c := newCanvas(TileSize, TileSize)
	base := groundBase[g]
	for y := 0; y < TileSize; y++ {
		for x := 0; x < TileSize; x++ {
			c.set(x, y, shade(base, 1+r.FRange(-0.05, 0.05)))
		}
	}
	switch g {
	case world.GGrass, world.GForest, world.GSwamp:
		for i := 0; i < 26; i++ {
			x, y := r.Intn(TileSize), r.Intn(TileSize-2)+2
			col := shade(base, r.FRange(1.12, 1.3))
			c.set(x, y, col)
			c.set(x, y-1, col)
			if r.Chance(0.4) {
				c.set(x+1, y-2, shade(base, 0.8))
			}
		}
		if g == world.GSwamp {
			for i := 0; i < 3; i++ {
				c.ellipse(r.FRange(6, 26), r.FRange(6, 26), r.FRange(2, 4), r.FRange(1, 2), color.NRGBA{60, 84, 70, 255}, false)
			}
		}
	case world.GWater, world.GDeepWater:
		for i := 0; i < 7; i++ {
			x, y := r.Intn(TileSize-6), r.Intn(TileSize)
			for k := 0; k < r.Range(3, 6); k++ {
				c.set(x+k, y, shade(base, 1.25))
			}
		}
	case world.GSand, world.GDirt, world.GPath, world.GAsh:
		for i := 0; i < 30; i++ {
			c.set(r.Intn(TileSize), r.Intn(TileSize), shade(base, r.FRange(0.8, 1.15)))
		}
		if g == world.GPath {
			for i := 0; i < 5; i++ {
				c.ellipse(r.FRange(4, 28), r.FRange(4, 28), r.FRange(2, 4), r.FRange(1.5, 3), shade(base, 0.88), true)
			}
		}
	case world.GSnow, world.GIce:
		for i := 0; i < 18; i++ {
			c.set(r.Intn(TileSize), r.Intn(TileSize), shade(base, r.FRange(0.9, 1.05)))
		}
		if g == world.GIce {
			c.line(r.Intn(32), r.Intn(32), r.Intn(32), r.Intn(32), shade(base, 1.12))
		}
	case world.GRock:
		for i := 0; i < 3; i++ {
			x, y := r.Intn(32), r.Intn(32)
			c.line(x, y, x+r.Range(-8, 8), y+r.Range(-8, 8), shade(base, 0.7))
		}
	case world.GLava:
		for i := 0; i < 6; i++ {
			c.ellipse(r.FRange(4, 28), r.FRange(4, 28), r.FRange(2, 5), r.FRange(1, 3), color.NRGBA{255, 190, 60, 255}, false)
		}
	case world.GDungeonFloor:
		line := shade(base, 0.75)
		for y := 0; y < TileSize; y += 16 {
			c.rect(0, y, TileSize-1, y, line)
		}
		c.rect(15, 0, 15, 15, line)
		c.rect(7, 16, 7, 31, line)
		c.rect(23, 16, 23, 31, line)
		for i := 0; i < 12; i++ {
			c.set(r.Intn(32), r.Intn(32), shade(base, r.FRange(0.85, 1.12)))
		}
	case world.GDungeonWall:
		mortar := shade(base, 0.6)
		for y := 0; y < TileSize; y += 8 {
			c.rect(0, y, TileSize-1, y, mortar)
			off := 0
			if (y/8)%2 == 1 {
				off = 8
			}
			for x := off; x < TileSize; x += 16 {
				c.rect(x, y, x, y+7, mortar)
			}
		}
		c.rect(0, 0, 31, 1, shade(base, 1.35))
	}
	return c.img
}

var (
	trunk   = color.NRGBA{104, 72, 44, 255}
	leaf    = color.NRGBA{58, 128, 58, 255}
	stone   = color.NRGBA{130, 128, 124, 255}
	darkOut = color.NRGBA{22, 20, 26, 255}
)

func objectTile(s uint64, o world.Object) *image.NRGBA {
	r := seed.New(s)
	c := newCanvas(TileSize, TileSize)
	switch o {
	case world.OTree:
		c.rect(14, 20, 17, 30, trunk)
		c.ellipse(16, 13, 12, 11, leaf, true)
		for i := 0; i < 10; i++ {
			c.set(r.Range(8, 24), r.Range(5, 20), shade(leaf, 1.3))
		}
	case world.OPine:
		c.rect(14, 24, 17, 30, trunk)
		for i, w := range []float64{6, 9, 12} {
			y := 6 + float64(i)*6
			for dy := 0.0; dy < 8; dy++ {
				half := int(w * (dy + 1) / 8)
				c.rect(16-half, int(y+dy), 15+half, int(y+dy), shade(color.NRGBA{40, 100, 70, 255}, 1.1-dy*0.04))
			}
		}
	case world.ORock:
		c.ellipse(16, 22, 9, 6, stone, true)
	case world.OBush:
		c.ellipse(16, 22, 9, 7, shade(leaf, 1.1), true)
		for i := 0; i < 4; i++ {
			c.set(r.Range(10, 22), r.Range(17, 26), color.NRGBA{220, 60, 70, 255})
		}
	case world.OFlowers:
		cols := []color.NRGBA{{240, 90, 110, 255}, {250, 220, 80, 255}, {170, 120, 240, 255}, {250, 250, 250, 255}}
		for i := 0; i < 14; i++ {
			x, y := r.Range(4, 27), r.Range(6, 28)
			c.set(x, y+1, shade(leaf, 0.8))
			c.set(x, y, seed.Pick(r, cols))
			c.set(x+1, y, seed.Pick(r, cols))
		}
		return c.img // no outline: flowers are ground decoration
	case world.OCactus:
		g := color.NRGBA{70, 150, 80, 255}
		c.rect(14, 8, 18, 29, g)
		c.rect(8, 14, 10, 20, g)
		c.rect(10, 19, 14, 21, g)
		c.rect(22, 11, 24, 17, g)
		c.rect(18, 16, 22, 18, g)
		c.rect(15, 8, 15, 28, shade(g, 1.25))
	case world.ODeadTree:
		c.rect(14, 12, 17, 30, trunk)
		c.line(15, 16, 7, 8, trunk)
		c.line(16, 14, 25, 6, trunk)
		c.line(16, 20, 23, 15, trunk)
	case world.OBoulder:
		c.ellipse(16, 19, 13, 11, shade(stone, 0.95), true)
		c.line(10, 16, 16, 20, shade(stone, 0.7))
	case world.OReeds:
		for i := 0; i < 7; i++ {
			x := r.Range(6, 26)
			h := r.Range(10, 20)
			c.line(x, 30, x+r.Range(-2, 2), 30-h, color.NRGBA{120, 150, 70, 255})
			c.set(x, 30-h, color.NRGBA{110, 80, 50, 255})
		}
		return c.img
	case world.ODungeonEntrance:
		c.ellipse(16, 18, 14, 13, shade(stone, 0.85), true)
		c.ellipse(16, 21, 8, 9, color.NRGBA{12, 10, 16, 255}, false)
		c.rect(8, 28, 24, 31, shade(stone, 0.7))
		c.set(12, 10, color.NRGBA{255, 200, 90, 255})
		c.set(20, 10, color.NRGBA{255, 200, 90, 255})
	case world.OStairsDown:
		for i := 0; i < 5; i++ {
			c.rect(6+i*2, 8+i*4, 25-i*2, 11+i*4, shade(stone, 1.1-float64(i)*0.15))
		}
	case world.ODungeonExit:
		c.rect(8, 4, 23, 30, shade(stone, 0.9))
		c.rect(11, 8, 20, 30, color.NRGBA{255, 236, 170, 255})
		c.rect(12, 10, 19, 30, color.NRGBA{255, 250, 220, 255})
	case world.OChest:
		wood := color.NRGBA{150, 96, 48, 255}
		c.rect(6, 12, 25, 27, wood)
		c.rect(6, 12, 25, 15, shade(wood, 1.25))
		c.rect(6, 18, 25, 19, color.NRGBA{210, 170, 60, 255})
		c.rect(14, 16, 17, 22, color.NRGBA{240, 200, 70, 255})
	case world.OTorch:
		c.rect(15, 14, 16, 28, trunk)
		c.ellipse(16, 10, 4, 6, color.NRGBA{255, 140, 30, 255}, false)
		c.ellipse(16, 11, 2, 3, color.NRGBA{255, 230, 120, 255}, false)
	case world.OAnvil:
		iron := color.NRGBA{80, 84, 96, 255}
		c.rect(6, 12, 26, 16, iron)
		c.rect(2, 12, 6, 13, iron)
		c.rect(12, 17, 20, 22, shade(iron, 0.8))
		c.rect(9, 23, 23, 27, shade(iron, 0.9))
		c.rect(6, 12, 26, 12, shade(iron, 1.5))
	case world.OShrine:
		c.rect(12, 6, 19, 28, color.NRGBA{200, 200, 215, 255})
		c.rect(9, 26, 22, 30, shade(stone, 0.9))
		c.ellipse(15.5, 12, 3, 3, color.NRGBA{120, 220, 255, 255}, false)
		c.rect(13, 6, 13, 26, color.NRGBA{235, 235, 245, 255})
	case world.OBones:
		b := color.NRGBA{226, 220, 200, 255}
		c.line(8, 22, 20, 26, b)
		c.line(12, 27, 24, 20, b)
		c.ellipse(22, 17, 4, 3.5, b, true)
		c.set(21, 17, darkOut)
		c.set(23, 17, darkOut)
	}
	c.outline(darkOut)
	return c.img
}
