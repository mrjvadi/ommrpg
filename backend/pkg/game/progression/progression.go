// Package progression holds the character rules: XP curve, attributes,
// derived combat stats, the hidden Root/Talent rolled at creation and class
// awakening at level 10 from accumulated behaviour.
package progression

import (
	"math"

	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

const (
	MaxLevel        = 100
	PointsPerLevel  = 4
	AwakeningLevel  = 10
	BaseAttribute   = 5
	BaseMoveSpeed   = 4.5 // tiles per second
	MaxMoveSpeedMul = 1.35
)

// XPToNext is the XP needed to go from level to level+1.
func XPToNext(level int) int64 {
	if level >= MaxLevel {
		return 0
	}
	return int64(math.Round(60 * math.Pow(float64(level), 1.65)))
}

// Attributes are the allocatable primary stats.
type Attributes struct {
	Str int `json:"str"`
	Agi int `json:"agi"`
	Int int `json:"int"`
	Vit int `json:"vit"`
}

func (a Attributes) Add(b Attributes) Attributes {
	return Attributes{a.Str + b.Str, a.Agi + b.Agi, a.Int + b.Int, a.Vit + b.Vit}
}

func (a Attributes) Sum() int { return a.Str + a.Agi + a.Int + a.Vit }

func (a Attributes) Valid() bool { return a.Str >= 0 && a.Agi >= 0 && a.Int >= 0 && a.Vit >= 0 }

// Root is the hidden origin of a character (see docs/CHARACTER_SYSTEM.md).
type Root string

const (
	Starforged   Root = "starforged"
	Voidborn     Root = "voidborn"
	SoulWeaver   Root = "soul_weaver"
	SpiritVessel Root = "spirit_vessel"
	BeastHeart   Root = "beast_heart"
	Mundane      Root = "mundane"
)

// Talent is the hidden innate gift.
type Talent string

const (
	PerfectSynthesis    Talent = "perfect_synthesis"
	SpatialAffinity     Talent = "spatial_affinity"
	BlacksmithPrecision Talent = "blacksmith_precision"
	FortuneTouched      Talent = "fortune_touched"
	QuickLearner        Talent = "quick_learner"
	Ironhide            Talent = "ironhide"
	NoTalent            Talent = "none"
)

// Hidden is never sent to the client as-is.
type Hidden struct {
	Root   Root   `json:"root"`
	Talent Talent `json:"talent"`
	// Potential multiplies XP gains (0.9..1.2).
	Potential float64 `json:"potential"`
}

// RollHidden derives a character's hidden traits from its seed.
func RollHidden(s uint64) Hidden {
	r := seed.New(seed.Derive(s, "hidden"))
	roots := []Root{Mundane, Starforged, Voidborn, SoulWeaver, SpiritVessel, BeastHeart}
	rootW := []float64{60, 8, 6, 8, 9, 9}
	talents := []Talent{NoTalent, PerfectSynthesis, SpatialAffinity, BlacksmithPrecision, FortuneTouched, QuickLearner, Ironhide}
	talW := []float64{40, 8, 8, 12, 10, 12, 10}
	return Hidden{
		Root:      roots[r.Weighted(rootW)],
		Talent:    talents[r.Weighted(talW)],
		Potential: math.Round(r.FRange(0.9, 1.2)*100) / 100,
	}
}

// RootBonus returns attribute bonuses granted by a root.
func RootBonus(root Root) Attributes {
	switch root {
	case Starforged:
		return Attributes{Str: 2, Vit: 2}
	case Voidborn:
		return Attributes{Int: 3, Agi: 1}
	case SoulWeaver:
		return Attributes{Int: 2, Vit: 1, Agi: 1}
	case SpiritVessel:
		return Attributes{Vit: 3, Int: 1}
	case BeastHeart:
		return Attributes{Str: 2, Agi: 2}
	}
	return Attributes{}
}

// XPMultiplier combines potential and talent.
func (h Hidden) XPMultiplier() float64 {
	m := h.Potential
	if h.Talent == QuickLearner {
		m *= 1.15
	}
	return m
}

// UpgradeBonus is added to enhancement success chance.
func (h Hidden) UpgradeBonus() float64 {
	switch h.Talent {
	case BlacksmithPrecision:
		return 0.06
	case PerfectSynthesis:
		return 0.03
	}
	return 0
}

// Luck is added to loot rarity rolls.
func (h Hidden) Luck() float64 {
	if h.Talent == FortuneTouched {
		return 0.25
	}
	return 0
}

// Bonuses from equipment, summed by the item-service.
type Bonuses struct {
	Attributes
	Damage     int     `json:"damage"` // flat weapon damage (avg)
	Magic      int     `json:"magic"`  // flat spell power
	Armor      int     `json:"armor"`
	HP         int     `json:"hp"`
	CritPct    float64 `json:"crit_pct"`
	SpeedPct   float64 `json:"speed_pct"`
	LifeSteal  float64 `json:"lifesteal_pct"`
	XPPct      float64 `json:"xp_pct"`
	MagicFind  float64 `json:"magic_find_pct"`
	WeaponKind string  `json:"weapon_kind"` // melee | ranged | magic | "" (fists)
	Range      float64 `json:"range"`
	Cooldown   float64 `json:"cooldown"`
}

// Derived are the numbers combat and movement actually use.
type Derived struct {
	MaxHP      int     `json:"max_hp"`
	Attack     int     `json:"attack"`
	Defense    int     `json:"defense"`
	CritChance float64 `json:"crit_chance"`
	Cooldown   float64 `json:"cooldown"`
	Range      float64 `json:"range"`
	MoveSpeed  float64 `json:"move_speed"`
	LifeSteal  float64 `json:"lifesteal"`
	XPBonus    float64 `json:"xp_bonus"`
	MagicFind  float64 `json:"magic_find"`
	WeaponKind string  `json:"weapon_kind"`
}

// Derive computes combat stats from level, attributes and equipment.
func Derive(level int, base Attributes, hidden Hidden, eq Bonuses) Derived {
	a := base.Add(RootBonus(hidden.Root)).Add(eq.Attributes)
	d := Derived{
		MaxHP:      60 + a.Vit*12 + level*8 + eq.HP,
		Defense:    a.Vit/2 + eq.Armor,
		CritChance: math.Min(0.6, 0.05+float64(a.Agi)*0.003+eq.CritPct/100),
		LifeSteal:  math.Min(0.25, eq.LifeSteal/100),
		XPBonus:    eq.XPPct / 100,
		MagicFind:  eq.MagicFind / 100,
		WeaponKind: eq.WeaponKind,
		Range:      eq.Range,
		Cooldown:   eq.Cooldown,
	}
	if hidden.Talent == Ironhide {
		d.Defense += 3 + level/5
	}
	switch eq.WeaponKind {
	case "magic":
		d.Attack = a.Int*2 + eq.Magic + eq.Damage/2
	case "ranged":
		d.Attack = a.Agi*2 + eq.Damage
	case "melee":
		d.Attack = a.Str*2 + eq.Damage
	default:
		d.Attack = a.Str + 2
		d.Range, d.Cooldown, d.WeaponKind = 1.4, 0.7, "melee"
	}
	if d.Range <= 0 {
		d.Range = 1.4
	}
	if d.Cooldown <= 0 {
		d.Cooldown = 0.7
	}
	d.Cooldown *= 1 - math.Min(float64(a.Agi)*0.003, 0.35)
	d.MoveSpeed = BaseMoveSpeed * math.Min(MaxMoveSpeedMul, 1+eq.SpeedPct/100)
	return d
}

// Behaviour counters accumulate from events and feed class awakening.
type Behaviour struct {
	MeleeKills   int `json:"melee_kills"`
	RangedKills  int `json:"ranged_kills"`
	MagicKills   int `json:"magic_kills"`
	Upgrades     int `json:"upgrades"`
	Salvages     int `json:"salvages"`
	DungeonClear int `json:"dungeon_clears"`
	Deaths       int `json:"deaths"`
}

// Class is awakened, never chosen.
type Class string

const (
	NoClass    Class = ""
	Warrior    Class = "warrior"
	Ranger     Class = "ranger"
	Mage       Class = "mage"
	Artisan    Class = "artisan"
	Warden     Class = "warden"
	Spellblade Class = "spellblade"
	// Hidden paths unlocked by rare roots.
	Starbreaker Class = "starbreaker"
	VoidWalker  Class = "void_walker"
	Soulbinder  Class = "soulbinder"
	Beastlord   Class = "beastlord"
)

// Awaken picks a class from what the character actually did.
func Awaken(attr Attributes, hidden Hidden, b Behaviour) Class {
	combat := b.MeleeKills + b.RangedKills + b.MagicKills
	craft := b.Upgrades*6 + b.Salvages*2
	var c Class
	switch {
	case craft > combat && craft >= 30:
		c = Artisan
	case b.MagicKills >= b.MeleeKills && b.MagicKills >= b.RangedKills:
		c = Mage
		if attr.Str > attr.Int {
			c = Spellblade
		}
	case b.RangedKills >= b.MeleeKills:
		c = Ranger
	default:
		c = Warrior
		if attr.Vit > attr.Str {
			c = Warden
		}
	}
	// Rare roots may awaken a hidden path when behaviour matches.
	switch {
	case hidden.Root == Starforged && (c == Warrior || c == Warden) && b.DungeonClear >= 1:
		return Starbreaker
	case hidden.Root == Voidborn && (c == Mage || c == Spellblade):
		return VoidWalker
	case hidden.Root == SoulWeaver && b.Deaths >= 3:
		return Soulbinder
	case hidden.Root == BeastHeart && c == Ranger:
		return Beastlord
	}
	return c
}
