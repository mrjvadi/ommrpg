package items

import (
	"math"

	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// Drop is one item rolled from a kill or chest. Its seed is derived from the
// kill so the same kill always yields the same drop (idempotent grants).
type Drop struct {
	Seed      uint64  `json:"seed,string"`
	ItemLevel int     `json:"item_level"`
	MinRarity Rarity  `json:"min_rarity"`
	Luck      float64 `json:"luck"`
}

// Loot is the full reward of a kill or chest.
type Loot struct {
	Gold    int64  `json:"gold"`
	Essence int64  `json:"essence"`
	Items   []Drop `json:"items"`
}

// RollLoot decides the reward. rank is "normal", "elite", "boss" or "chest".
func RollLoot(s uint64, level int, rank string, luck float64) Loot {
	r := seed.New(seed.Derive(s, "loot"))
	l := Loot{Gold: int64(float64(level) * r.FRange(2, 6))}
	n, minR := 0, Common
	switch rank {
	case "elite":
		n = r.Range(1, 2)
		l.Gold *= 3
		l.Essence = int64(r.Range(1, 3))
		minR = Uncommon
	case "boss":
		n = r.Range(3, 5)
		l.Gold *= 10
		l.Essence = int64(r.Range(5, 10))
		minR = Rare
	case "chest":
		n = r.Range(1, 3)
		l.Gold *= 4
		l.Essence = int64(r.Range(2, 5))
	default:
		if r.Chance(0.3 + math.Min(0.3, luck*0.3)) {
			n = 1
		}
		if r.Chance(0.15) {
			l.Essence = 1
		}
	}
	for i := 0; i < n; i++ {
		ilvl := level + r.Range(-1, 1)
		if ilvl < 1 {
			ilvl = 1
		}
		l.Items = append(l.Items, Drop{Seed: seed.Derive(s, "drop", i), ItemLevel: ilvl, MinRarity: minR, Luck: luck})
	}
	return l
}
