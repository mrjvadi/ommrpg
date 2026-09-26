// Package bestiary generates the monster species of a world from its seed and
// places monster spawns in overworld chunks. Every world has its own
// procedurally named species with their own stats, colours and sprite seeds.
package bestiary

import (
	"fmt"
	"sync"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/names"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// Family decides a species' silhouette (sprite template), stat profile and
// where it lives. The string values are used by the sprite-service.
type Family string

const (
	Slime     Family = "slime"
	Beast     Family = "beast"
	Insect    Family = "insect"
	Bird      Family = "bird"
	Undead    Family = "undead"
	Elemental Family = "elemental"
	Plant     Family = "plant"
	Golem     Family = "golem"
	Serpent   Family = "serpent"
	Imp       Family = "imp"
	Spirit    Family = "spirit"
)

type familyDef struct {
	fam          Family
	biomes       []world.Biome
	nouns        []string
	hp, atk, def float64
	ranged       bool
	speed        float64 // move speed tiles/s (used by client wander + future AI)
}

var families = []familyDef{
	{Slime, []world.Biome{world.BPlains, world.BForest, world.BSwamp, world.BDungeon}, []string{"Ooze", "Slime", "Jelly", "Blob"}, 1.1, 0.8, 0.8, false, 1.2},
	{Beast, []world.Biome{world.BPlains, world.BForest, world.BTaiga, world.BTundra}, []string{"Wolf", "Boar", "Stalker", "Hound", "Bear"}, 1.0, 1.1, 1.0, false, 2.4},
	{Insect, []world.Biome{world.BForest, world.BDesert, world.BSwamp, world.BDungeon}, []string{"Beetle", "Mantis", "Crawler", "Scarab"}, 0.8, 1.2, 1.2, false, 2.0},
	{Bird, []world.Biome{world.BPlains, world.BBeach, world.BMountain}, []string{"Hawk", "Raptor", "Harpy", "Gull"}, 0.7, 1.2, 0.6, true, 3.0},
	{Undead, []world.Biome{world.BSwamp, world.BVolcanic, world.BDungeon, world.BTundra}, []string{"Skeleton", "Ghoul", "Wight", "Revenant"}, 1.0, 1.1, 1.0, false, 1.4},
	{Elemental, []world.Biome{world.BVolcanic, world.BMountain, world.BTundra, world.BDesert}, []string{"Wisp", "Elemental", "Spark", "Shard"}, 0.9, 1.3, 0.9, true, 1.8},
	{Plant, []world.Biome{world.BForest, world.BSwamp, world.BPlains}, []string{"Bloom", "Thorn", "Mandrake", "Creeper"}, 1.3, 0.9, 1.1, true, 0.6},
	{Golem, []world.Biome{world.BMountain, world.BDesert, world.BDungeon}, []string{"Golem", "Colossus", "Sentinel"}, 1.7, 1.0, 1.6, false, 1.0},
	{Serpent, []world.Biome{world.BDesert, world.BSwamp, world.BBeach}, []string{"Serpent", "Viper", "Naga", "Wyrm"}, 1.0, 1.2, 0.9, false, 2.2},
	{Imp, []world.Biome{world.BVolcanic, world.BDungeon}, []string{"Imp", "Fiend", "Devil"}, 0.8, 1.4, 0.8, true, 2.6},
	{Spirit, []world.Biome{world.BTundra, world.BTaiga, world.BDungeon}, []string{"Spirit", "Wraith", "Shade", "Phantom"}, 0.8, 1.3, 0.7, true, 2.0},
}

// familyResist: damage multipliers per element (missing = 1.0).
var familyResist = map[Family]map[string]float64{
	Plant:     {"fire": 1.6, "poison": 0.5, "frost": 1.2},
	Undead:    {"holy": 1.8, "shadow": 0.4, "poison": 0.3},
	Beast:     {"fire": 1.2, "poison": 1.3},
	Insect:    {"fire": 1.4, "frost": 1.3},
	Golem:     {"lightning": 1.5, "poison": 0.2, "fire": 0.7},
	Slime:     {"lightning": 1.4, "frost": 1.3, "poison": 0.5},
	Bird:      {"lightning": 1.5, "frost": 1.2},
	Serpent:   {"frost": 1.5, "poison": 0.4},
	Imp:       {"holy": 1.6, "fire": 0.3, "frost": 1.4},
	Spirit:    {"holy": 1.5, "shadow": 1.3, "poison": 0.2},
	Elemental: {},
}

var opposite = map[string]string{"fire": "frost", "frost": "fire", "holy": "shadow", "shadow": "holy", "lightning": "poison", "poison": "lightning"}

// Resist returns the damage multiplier of an element against a species.
// Elemental-aligned species are nearly immune to their own element and
// weak to its opposite.
func (s Species) Resist(element string) float64 {
	if element == "" {
		return 1
	}
	if s.Element != "" {
		if element == s.Element {
			return 0.2
		}
		if opposite[s.Element] == element {
			return 1.8
		}
	}
	if m, ok := familyResist[s.Family][element]; ok {
		return m
	}
	return 1
}

// Species is one generated monster type.
type Species struct {
	ID         int           `json:"id"`
	Name       string        `json:"name"`
	Family     Family        `json:"family"`
	Biomes     []world.Biome `json:"biomes"`
	HPMul      float64       `json:"hp_mul"`
	AtkMul     float64       `json:"atk_mul"`
	DefMul     float64       `json:"def_mul"`
	XPMul      float64       `json:"xp_mul"`
	Ranged     bool          `json:"ranged"`
	Range      float64       `json:"range"`
	Speed      float64       `json:"speed"`
	SpriteSeed uint64        `json:"sprite_seed,string"`
	Hue        float64       `json:"hue"`
	Hue2       float64       `json:"hue2"`
	Big        bool          `json:"big"`
	// Element alignment (elementals, imps and spirits only).
	Element string `json:"element,omitempty"`
}

const PerFamily = 4

var cache sync.Map // worldSeed -> []Species

// ForWorld returns all species of a world (cached; pure function of seed).
func ForWorld(worldSeed uint64) []Species {
	if v, ok := cache.Load(worldSeed); ok {
		return v.([]Species)
	}
	var out []Species
	for fi, f := range families {
		for k := 0; k < PerFamily; k++ {
			id := fi*PerFamily + k
			r := seed.New(seed.Derive(worldSeed, "species", id))
			// each species lives in 1-2 of its family's biomes (dungeon always kept if present)
			biomes := []world.Biome{}
			perm := append([]world.Biome(nil), f.biomes...)
			seed.Shuffle(r, perm)
			n := 1 + r.Intn(2)
			if n > len(perm) {
				n = len(perm)
			}
			biomes = append(biomes, perm[:n]...)
			s := Species{
				ID:         id,
				Name:       fmt.Sprintf("%s %s", names.Word(r, r.Range(1, 2)), seed.Pick(r, f.nouns)),
				Family:     f.fam,
				Biomes:     biomes,
				HPMul:      f.hp * r.FRange(0.85, 1.2),
				AtkMul:     f.atk * r.FRange(0.85, 1.2),
				DefMul:     f.def * r.FRange(0.85, 1.2),
				Ranged:     f.ranged,
				Speed:      f.speed * r.FRange(0.8, 1.2),
				SpriteSeed: r.Uint64(),
				Hue:        r.Float(),
				Hue2:       r.Float(),
				Big:        r.Chance(0.2),
			}
			s.XPMul = (s.HPMul + s.AtkMul + s.DefMul) / 3
			if f.fam == Elemental || f.fam == Imp || f.fam == Spirit {
				er := seed.New(seed.Derive(worldSeed, "species-element", id))
				switch f.fam {
				case Imp:
					s.Element = seed.Pick(er, []string{"fire", "shadow"})
				case Spirit:
					s.Element = seed.Pick(er, []string{"frost", "shadow", "holy"})
				default:
					s.Element = seed.Pick(er, []string{"fire", "frost", "lightning", "poison"})
				}
			}
			if s.Big {
				s.HPMul *= 1.4
				s.XPMul *= 1.3
			}
			s.Range = 1.4
			if s.Ranged {
				s.Range = 4.5
			}
			out = append(out, s)
		}
	}
	cache.Store(worldSeed, out)
	return out
}

// Get returns one species by id.
func Get(worldSeed uint64, id int) (Species, bool) {
	all := ForWorld(worldSeed)
	if id < 0 || id >= len(all) {
		return Species{}, false
	}
	return all[id], true
}

// InBiome lists species that live in a biome.
func InBiome(worldSeed uint64, b world.Biome) []Species {
	var out []Species
	for _, s := range ForWorld(worldSeed) {
		for _, sb := range s.Biomes {
			if sb == b {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// Rank of a monster instance.
type Rank string

const (
	Normal Rank = "normal"
	Elite  Rank = "elite"
	Boss   Rank = "boss"
)

// Spawn is one monster placed in a zone. It is fully determined by the zone
// seed; its mutable state (hp, death timer) lives in the combat-service.
type Spawn struct {
	ID      string `json:"id"`
	Species int    `json:"species"`
	Level   int    `json:"level"`
	Rank    Rank   `json:"rank"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

// Stats are the combat numbers of a monster instance.
type Stats struct {
	MaxHP    int     `json:"max_hp"`
	Attack   int     `json:"attack"`
	Defense  int     `json:"defense"`
	XP       int     `json:"xp"`
	Range    float64 `json:"range"`
	Cooldown float64 `json:"cooldown"`
	Respawn  int     `json:"respawn_seconds"`
}

func rankMul(r Rank) (hp, atk, xp float64) {
	switch r {
	case Elite:
		return 2.6, 1.4, 3
	case Boss:
		return 9, 1.8, 12
	}
	return 1, 1, 1
}

// StatsFor computes monster stats for a spawn.
func StatsFor(s Species, level int, rank Rank) Stats {
	L := float64(level)
	hm, am, xm := rankMul(rank)
	st := Stats{
		MaxHP:    int((34 + 20*L) * s.HPMul * hm),
		Attack:   int((4 + 2.4*L) * s.AtkMul * am),
		Defense:  int((1 + 1.3*L) * s.DefMul),
		XP:       int((12 + 7*L) * s.XPMul * xm),
		Range:    s.Range,
		Cooldown: 1.6,
		Respawn:  45,
	}
	switch rank {
	case Elite:
		st.Respawn = 180
	case Boss:
		st.Respawn = 0 // bosses live in dungeon instances and do not respawn
		st.Cooldown = 1.3
	}
	return st
}

var biomeDensity = map[world.Biome]int{
	world.BPlains: 2, world.BForest: 3, world.BTaiga: 3, world.BTundra: 2, world.BDesert: 2,
	world.BSwamp: 3, world.BMountain: 2, world.BVolcanic: 3, world.BBeach: 1,
}

// ChunkSpawns returns the monsters living in an overworld chunk.
func ChunkSpawns(p world.Params, cx, cy int) []Spawn {
	r := seed.New(seed.Derive(p.Seed, "spawns", cx, cy))
	cxT, cyT := cx*world.ChunkSize+world.ChunkSize/2, cy*world.ChunkSize+world.ChunkSize/2
	b := p.BiomeAt(cxT, cyT)
	base, ok := biomeDensity[b]
	if !ok {
		return nil
	}
	count := r.Range(base-1, base+1)
	var out []Spawn
	for i := 0; i < count; i++ {
		x, y, ok := p.FreeTileIn(cx, cy, r, 10)
		if !ok {
			continue
		}
		tb := p.BiomeAt(x, y)
		cands := InBiome(p.Seed, tb)
		if len(cands) == 0 {
			continue
		}
		sp := seed.Pick(r, cands)
		lvl := p.LevelAt(x, y) + r.Range(-1, 1)
		if lvl < 1 {
			lvl = 1
		}
		rank := Normal
		if lvl > 2 && r.Chance(0.07) {
			rank = Elite
		}
		out = append(out, Spawn{
			ID:      fmt.Sprintf("w%d_%d_%d_%d", p.ID, cx, cy, i),
			Species: sp.ID,
			Level:   lvl,
			Rank:    rank,
			X:       x,
			Y:       y,
		})
	}
	return out
}
