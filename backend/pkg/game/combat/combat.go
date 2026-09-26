// Package combat holds the damage formulas. Services pass in a seed derived
// from (server secret, attacker, attack sequence) so results are auditable.
package combat

import (
	"math"

	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// Hit is the outcome of one attack.
type Hit struct {
	Damage  int     `json:"damage"`
	Crit    bool    `json:"crit"`
	Roll    float64 `json:"roll"`
	Element string  `json:"element,omitempty"`
	Bonus   int     `json:"bonus,omitempty"` // elemental part of Damage
}

// Resolve computes damage of attack vs defense.
func Resolve(s uint64, attack, defense int, critChance float64) Hit {
	r := seed.New(s)
	raw := float64(attack) * r.FRange(0.85, 1.15)
	h := Hit{Roll: r.Float()}
	if h.Roll < critChance {
		h.Crit = true
		raw *= 1.75
	}
	dmg := raw * 60 / (60 + float64(defense))
	h.Damage = int(math.Max(1, math.Round(dmg)))
	return h
}

// InRange compares positions (tile units, centre of tile = +0.5).
func InRange(ax, ay, bx, by, rng float64) bool {
	return math.Hypot(ax-bx, ay-by) <= rng+0.35
}
