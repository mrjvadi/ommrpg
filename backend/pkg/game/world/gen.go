// Package world generates the overworld terrain as a pure function of the
// world seed. Nothing here is stored: any service (world, presence, combat)
// can recompute any tile at any time and always gets the same answer.
package world

import (
	"fmt"
	"math"
	"sync"

	"github.com/mrjvadi/ommrpg/backend/pkg/noise"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

const (
	ChunkSize = 32
	// SafeRadius around the spawn point: no monsters, no dungeon entrances.
	SafeRadius = 14
	// PlazaRadius is the paved starter plaza around the spawn point.
	PlazaRadius = 4
)

// Params fully describe a world. Only these values are persisted.
type Params struct {
	ID         int    `json:"id"`
	Seed       uint64 `json:"seed,string"`
	SizeChunks int    `json:"size_chunks"`
	Name       string `json:"name"`
}

func (p Params) SizeTiles() int { return p.SizeChunks * ChunkSize }

// InBounds reports whether a tile coordinate lies inside the world.
func (p Params) InBounds(x, y int) bool {
	n := p.SizeTiles()
	return x >= 0 && y >= 0 && x < n && y < n
}

// Sample is the raw terrain classification of one tile.
type Sample struct {
	Ground      Ground
	Biome       Biome
	Elevation   float64
	Moisture    float64
	Temperature float64
}

func smoothstep(a, b, x float64) float64 {
	t := (x - a) / (b - a)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// SampleAt classifies terrain at tile (x,y) ignoring props and features.
func (p Params) SampleAt(x, y int) Sample {
	if !p.InBounds(x, y) {
		return Sample{Ground: GDeepWater, Biome: BOcean}
	}
	size := float64(p.SizeTiles())
	fx, fy := float64(x), float64(y)
	e := noise.FBM(seed.Derive(p.Seed, "elev"), fx/96, fy/96, 5, 2, 0.5)
	dx, dy := fx/size-0.5, fy/size-0.5
	d := math.Sqrt(dx*dx+dy*dy) * 2 // 0 center, 1 edge midpoint
	e = e + 0.17 - smoothstep(0.62, 1.05, d)*0.7
	t := noise.FBM(seed.Derive(p.Seed, "temp"), fx/420, fy/420, 3, 2, 0.5)
	// North is cold, south is hot; high ground is colder.
	t = (t-0.5)*0.9 + 0.5 + (fy/size-0.5)*0.75 - math.Max(0, e-0.6)*0.5
	m := noise.FBM(seed.Derive(p.Seed, "moist"), fx/200, fy/200, 4, 2, 0.5)
	m = (m-0.5)*1.4 + 0.5
	// Ridged noise draws long mountain ranges across the continent.
	ridge := 1 - math.Abs(noise.Perlin(seed.Derive(p.Seed, "ridge"), fx/150, fy/150))
	mask := noise.FBM(seed.Derive(p.Seed, "ridge-mask"), fx/300, fy/300, 2, 2, 0.5)
	if ridge > 0.955 && e > 0.44 && mask > 0.5 {
		e = math.Max(e, 0.81+(ridge-0.955))
	}
	s := Sample{Elevation: e, Moisture: m, Temperature: t}
	switch {
	case e < 0.30:
		s.Ground, s.Biome = GDeepWater, BOcean
	case e < 0.36:
		s.Ground, s.Biome = GWater, BOcean
	case e < 0.39:
		s.Ground, s.Biome = GSand, BBeach
	case e > 0.80:
		s.Biome = BMountain
		if t < 0.42 {
			s.Ground = GSnow
		} else {
			s.Ground = GRock
		}
	case t < 0.33:
		if m > 0.5 {
			s.Ground, s.Biome = GForest, BTaiga
		} else if m < 0.36 {
			s.Ground, s.Biome = GIce, BTundra
		} else {
			s.Ground, s.Biome = GSnow, BTundra
		}
	case t > 0.70 && m < 0.34 && e > 0.58:
		s.Ground, s.Biome = GAsh, BVolcanic
	case t > 0.64 && m < 0.46:
		s.Ground, s.Biome = GSand, BDesert
	case m > 0.68:
		s.Ground, s.Biome = GSwamp, BSwamp
	case m > 0.52:
		s.Ground, s.Biome = GForest, BForest
	default:
		s.Biome = BPlains
		if m < 0.36 {
			s.Ground = GDirt
		} else {
			s.Ground = GGrass
		}
	}
	return s
}

type propRule struct {
	obj  Object
	prob float64
}

var props = map[Biome][]propRule{
	BPlains:   {{OTree, 0.025}, {OBush, 0.03}, {OFlowers, 0.05}, {ORock, 0.008}},
	BForest:   {{OTree, 0.22}, {OBush, 0.06}, {OFlowers, 0.02}, {ORock, 0.01}},
	BTaiga:    {{OPine, 0.2}, {ORock, 0.02}, {OBush, 0.02}},
	BTundra:   {{OPine, 0.03}, {ORock, 0.03}},
	BDesert:   {{OCactus, 0.03}, {ORock, 0.02}, {ODeadTree, 0.008}},
	BSwamp:    {{OReeds, 0.1}, {ODeadTree, 0.04}, {OTree, 0.03}},
	BMountain: {{OBoulder, 0.1}, {ORock, 0.06}},
	BVolcanic: {{ORock, 0.06}, {ODeadTree, 0.03}},
	BBeach:    {{ORock, 0.01}},
}

// baseTile is terrain + natural props, before chunk features and the plaza.
func (p Params) baseTile(x, y int) (Ground, Object, Biome) {
	s := p.SampleAt(x, y)
	rules, ok := props[s.Biome]
	if !ok {
		return s.Ground, ONone, s.Biome
	}
	cluster := 0.4 + 1.2*noise.FBM(seed.Derive(p.Seed, "cluster"), float64(x)/24, float64(y)/24, 2, 2, 0.5)
	r := seed.New(seed.Derive(p.Seed, "prop", x, y)).Float()
	acc := 0.0
	for _, rule := range rules {
		acc += rule.prob * cluster
		if r < acc {
			return s.Ground, rule.obj, s.Biome
		}
	}
	return s.Ground, ONone, s.Biome
}

var spawnCache sync.Map // map[uint64]([2]int)

// Spawn returns the starter point: the walkable tile closest to the world
// centre (spiral search). Cached per seed since it never changes.
func (p Params) Spawn() (int, int) {
	key := p.Seed ^ uint64(p.SizeChunks)<<56
	if v, ok := spawnCache.Load(key); ok {
		pt := v.([2]int)
		return pt[0], pt[1]
	}
	c := p.SizeTiles() / 2
	best := [2]int{c, c}
	found := false
	for r := 0; r < p.SizeTiles()/2 && !found; r++ {
		for dx := -r; dx <= r && !found; dx++ {
			for _, dy := range []int{-r, r} {
				x, y := c+dx, c+dy
				if p.plazaFits(x, y) {
					best, found = [2]int{x, y}, true
					break
				}
			}
		}
		for dy := -r + 1; dy <= r-1 && !found; dy++ {
			for _, dx := range []int{-r, r} {
				x, y := c+dx, c+dy
				if p.plazaFits(x, y) {
					best, found = [2]int{x, y}, true
					break
				}
			}
		}
	}
	spawnCache.Store(key, best)
	return best[0], best[1]
}

// plazaFits: the plaza needs solid land (grass/dirt/sand/forest) around it.
func (p Params) plazaFits(x, y int) bool {
	for dy := -PlazaRadius - 1; dy <= PlazaRadius+1; dy++ {
		for dx := -PlazaRadius - 1; dx <= PlazaRadius+1; dx++ {
			s := p.SampleAt(x+dx, y+dy)
			switch s.Ground {
			case GGrass, GDirt, GForest, GSand, GSnow:
			default:
				return false
			}
		}
	}
	return true
}

// DistToSpawn in tiles.
func (p Params) DistToSpawn(x, y int) float64 {
	sx, sy := p.Spawn()
	return math.Hypot(float64(x-sx), float64(y-sy))
}

// LevelAt is the danger level of a location: grows with distance from spawn.
func (p Params) LevelAt(x, y int) int {
	l := 1 + int(p.DistToSpawn(x, y)/28)
	if l > 60 {
		l = 60
	}
	return l
}

// Entrance is a dungeon entrance placed by the world generator.
type Entrance struct {
	ID    string `json:"id"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Tier  int    `json:"tier"`
	Name  string `json:"name"`
	Biome Biome  `json:"biome"`
}

// ChunkOf returns the chunk coordinate containing tile (x,y).
func ChunkOf(x, y int) (int, int) {
	return floorDiv(x, ChunkSize), floorDiv(y, ChunkSize)
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// FreeTileIn deterministically picks a walkable, empty, non-safe-zone tile in
// the chunk for the given purpose. ok=false when none was found.
func (p Params) FreeTileIn(cx, cy int, r *seed.Rand, attempts int) (int, int, bool) {
	for i := 0; i < attempts; i++ {
		x := cx*ChunkSize + r.Intn(ChunkSize)
		y := cy*ChunkSize + r.Intn(ChunkSize)
		if !p.InBounds(x, y) {
			continue
		}
		g, o, _ := p.baseTile(x, y)
		if !g.Walkable() || o != ONone || p.DistToSpawn(x, y) < SafeRadius {
			continue
		}
		return x, y, true
	}
	return 0, 0, false
}

// EntranceIn returns the dungeon entrance of a chunk, if it has one.
func (p Params) EntranceIn(cx, cy int) (Entrance, bool) {
	n := p.SizeChunks
	if cx < 0 || cy < 0 || cx >= n || cy >= n {
		return Entrance{}, false
	}
	r := seed.New(seed.Derive(p.Seed, "entrance", cx, cy))
	if !r.Chance(0.07) {
		return Entrance{}, false
	}
	x, y, ok := p.FreeTileIn(cx, cy, r, 16)
	if !ok || p.DistToSpawn(x, y) < SafeRadius+6 {
		return Entrance{}, false
	}
	_, _, b := p.baseTile(x, y)
	lvl := p.LevelAt(x, y)
	return Entrance{
		ID:    fmt.Sprintf("%d_%d_%d", p.ID, cx, cy),
		X:     x,
		Y:     y,
		Tier:  1 + lvl/6,
		Biome: b,
		Name:  dungeonName(seed.Derive(p.Seed, "dname", cx, cy), b),
	}, true
}

// Tile returns the final ground/object of any overworld tile, including the
// plaza and dungeon entrances. This is what movement validation uses.
func (p Params) Tile(x, y int) (Ground, Object) {
	cx, cy := ChunkOf(x, y)
	e, ok := p.EntranceIn(cx, cy)
	if !ok {
		e.X = -1
	}
	return p.tileWith(x, y, e)
}

func (p Params) tileWith(x, y int, ent Entrance) (Ground, Object) {
	sx, sy := p.Spawn()
	dx, dy := x-sx, y-sy
	if dx*dx+dy*dy <= (PlazaRadius+2)*(PlazaRadius+2) {
		g, _, _ := p.baseTile(x, y)
		if dx*dx+dy*dy <= PlazaRadius*PlazaRadius {
			g = GPath
		}
		switch {
		case dx == 0 && dy == 0:
			return g, OShrine
		case dx == 2 && dy == -2:
			return g, OAnvil
		case (dx == -PlazaRadius || dx == PlazaRadius) && dy == -PlazaRadius+1:
			return g, OTorch
		}
		return g, ONone
	}
	g, o, b := p.baseTile(x, y)
	if ent.X == x && ent.Y == y {
		return g, ODungeonEntrance
	}
	// The world edge is always water so nobody walks off the map.
	if b == BOcean || !p.InBounds(x, y) {
		return g, ONone
	}
	return g, o
}

// Walkable is the authoritative collision test for the overworld.
func (p Params) Walkable(x, y int) bool {
	if !p.InBounds(x, y) {
		return false
	}
	g, o := p.Tile(x, y)
	return g.Walkable() && !o.Blocking()
}

// BiomeAt returns the biome of a tile.
func (p Params) BiomeAt(x, y int) Biome { return p.SampleAt(x, y).Biome }

// Terrain is the tile payload of one chunk.
type Terrain struct {
	CX      int    `json:"cx"`
	CY      int    `json:"cy"`
	Size    int    `json:"size"`
	Ground  []byte `json:"ground"`
	Objects []byte `json:"objects"`
	Biomes  []byte `json:"biomes"`
}

// ChunkTerrain generates the tiles of chunk (cx,cy), row-major.
func (p Params) ChunkTerrain(cx, cy int) Terrain {
	t := Terrain{CX: cx, CY: cy, Size: ChunkSize,
		Ground:  make([]byte, ChunkSize*ChunkSize),
		Objects: make([]byte, ChunkSize*ChunkSize),
		Biomes:  make([]byte, ChunkSize*ChunkSize),
	}
	ent, ok := p.EntranceIn(cx, cy)
	if !ok {
		ent.X = -1
	}
	for ly := 0; ly < ChunkSize; ly++ {
		for lx := 0; lx < ChunkSize; lx++ {
			x, y := cx*ChunkSize+lx, cy*ChunkSize+ly
			g, o := p.tileWith(x, y, ent)
			i := ly*ChunkSize + lx
			t.Ground[i], t.Objects[i] = byte(g), byte(o)
			t.Biomes[i] = byte(p.BiomeAt(x, y))
		}
	}
	return t
}

var dungeonNouns = map[Biome][]string{
	BPlains:   {"Barrow", "Crypt", "Warren", "Hollow"},
	BForest:   {"Thicket", "Hollow", "Grove", "Den"},
	BTaiga:    {"Den", "Lair", "Hollow"},
	BTundra:   {"Glacier Vault", "Frost Cave", "Ice Tomb"},
	BDesert:   {"Tomb", "Sunken Temple", "Sand Crypt"},
	BSwamp:    {"Mire Pit", "Bog Crypt", "Sunken Ruin"},
	BMountain: {"Mine", "Deep Hall", "Cavern"},
	BVolcanic: {"Forge", "Magma Vault", "Ember Pit"},
	BBeach:    {"Grotto", "Sea Cave"},
}

func dungeonName(s uint64, b Biome) string {
	r := seed.New(s)
	nouns, ok := dungeonNouns[b]
	if !ok {
		nouns = []string{"Cave"}
	}
	return fmt.Sprintf("%s of %s", seed.Pick(r, nouns), nameWord(r))
}
