// Package items generates equipment procedurally from a seed and holds the
// rules for item leveling, enhancement (+N upgrades) and salvage.
//
// An item is born as Generate(seed, itemLevel, luck). The item-service stores
// the seed *and* a snapshot of the generated result, so later balance changes
// never silently rewrite items players already own.
package items

import (
	"fmt"
	"math"
	"strings"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/names"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// RulesVersion is stored with every generated item snapshot.
const RulesVersion = 1

type Slot string

const (
	Weapon  Slot = "weapon"
	Offhand Slot = "offhand"
	Head    Slot = "head"
	Chest   Slot = "chest"
	Legs    Slot = "legs"
	Feet    Slot = "feet"
	Hands   Slot = "hands"
	Ring    Slot = "ring"
	Amulet  Slot = "amulet"
)

var AllSlots = []Slot{Weapon, Offhand, Head, Chest, Legs, Feet, Hands, Ring, Amulet}

func ValidSlot(s string) bool {
	for _, x := range AllSlots {
		if string(x) == s {
			return true
		}
	}
	return false
}

type Rarity int

const (
	Common Rarity = iota
	Uncommon
	Rare
	Epic
	Legendary
	Mythic
)

var rarityNames = []string{"common", "uncommon", "rare", "epic", "legendary", "mythic"}

func (r Rarity) String() string { return rarityNames[r] }

func (r Rarity) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

func (r *Rarity) UnmarshalText(b []byte) error {
	for i, n := range rarityNames {
		if n == string(b) {
			*r = Rarity(i)
			return nil
		}
	}
	return fmt.Errorf("unknown rarity %q", b)
}

// AffixCount per rarity.
func (r Rarity) AffixCount() int { return []int{0, 1, 2, 3, 4, 5}[r] }

// MaxLevel is how far an item can level up by being used.
func (r Rarity) MaxLevel() int { return []int{10, 15, 20, 30, 40, 50}[r] }

// MaxEnhance is the highest +N an item can be upgraded to.
func (r Rarity) MaxEnhance() int { return []int{5, 7, 10, 12, 15, 20}[r] }

// Value multiplier used by costs and salvage.
func (r Rarity) Value() float64 { return []float64{1, 1.6, 2.6, 4.2, 7, 12}[r] }

// Base is an item archetype.
type Base struct {
	ID       string  `json:"id"`
	Slot     Slot    `json:"slot"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind,omitempty"` // weapons: melee|ranged|magic
	DmgMul   float64 `json:"-"`
	ArmorMul float64 `json:"-"`
	Cooldown float64 `json:"cooldown,omitempty"`
	Range    float64 `json:"range,omitempty"`
	CritPct  float64 `json:"-"`
	Icon     string  `json:"icon"` // procedural icon shape used by sprite-service
	// Appearance options on the LPC character (sprite-service catalog ids).
	Looks    []string `json:"-"`
	Material string   `json:"-"` // metal | cloth | wood | ""
}

var Bases = []Base{
	{ID: "sword", Slot: Weapon, Name: "Sword", Kind: "melee", DmgMul: 1.0, Cooldown: 0.6, Range: 1.6, Icon: "sword", Looks: []string{"weapon_sword_arming", "weapon_sword_longsword", "weapon_sword_saber"}, Material: "metal"},
	{ID: "axe", Slot: Weapon, Name: "Axe", Kind: "melee", DmgMul: 1.3, Cooldown: 0.85, Range: 1.5, Icon: "axe", Looks: []string{"weapon_blunt_waraxe"}, Material: "metal"},
	{ID: "mace", Slot: Weapon, Name: "Mace", Kind: "melee", DmgMul: 1.18, Cooldown: 0.75, Range: 1.5, Icon: "mace", Looks: []string{"weapon_blunt_mace", "weapon_blunt_flail"}, Material: "metal"},
	{ID: "spear", Slot: Weapon, Name: "Spear", Kind: "melee", DmgMul: 1.1, Cooldown: 0.7, Range: 2.3, Icon: "spear", Looks: []string{"weapon_polearm_spear", "weapon_polearm_halberd"}, Material: "metal"},
	{ID: "dagger", Slot: Weapon, Name: "Dagger", Kind: "melee", DmgMul: 0.72, Cooldown: 0.42, Range: 1.3, CritPct: 8, Icon: "dagger", Looks: []string{"weapon_sword_dagger"}, Material: "metal"},
	{ID: "bow", Slot: Weapon, Name: "Bow", Kind: "ranged", DmgMul: 0.95, Cooldown: 0.8, Range: 6, Icon: "bow", Looks: []string{"weapon_ranged_bow_normal", "weapon_ranged_bow_recurve", "weapon_ranged_bow_great"}, Material: "wood"},
	{ID: "crossbow", Slot: Weapon, Name: "Crossbow", Kind: "ranged", DmgMul: 1.25, Cooldown: 1.15, Range: 6.5, Icon: "crossbow", Looks: []string{"weapon_ranged_crossbow"}, Material: "wood"},
	{ID: "staff", Slot: Weapon, Name: "Staff", Kind: "magic", DmgMul: 1.15, Cooldown: 0.95, Range: 5, Icon: "staff", Looks: []string{"weapon_magic_gnarled", "weapon_magic_crystal", "weapon_magic_simple"}, Material: "wood"},
	{ID: "wand", Slot: Weapon, Name: "Wand", Kind: "magic", DmgMul: 0.78, Cooldown: 0.5, Range: 5, Icon: "wand", Looks: []string{"weapon_magic_wand"}, Material: "wood"},
	{ID: "shield", Slot: Offhand, Name: "Shield", ArmorMul: 1.4, Icon: "shield", Looks: []string{"shield_round", "shield_kite", "shield_spartan"}, Material: "metal"},
	{ID: "tome", Slot: Offhand, Name: "Tome", ArmorMul: 0.3, Icon: "tome"},
	{ID: "helm", Slot: Head, Name: "Helm", ArmorMul: 1.0, Icon: "helm", Looks: []string{"hat_helmet_nasal", "hat_helmet_barbuta", "hat_helmet_greathelm", "hat_helmet_horned", "hat_helmet_kettle"}, Material: "metal"},
	{ID: "hood", Slot: Head, Name: "Hood", ArmorMul: 0.55, Icon: "hood", Looks: []string{"hat_hood_cloth"}, Material: "cloth"},
	{ID: "plate", Slot: Chest, Name: "Plate Armor", ArmorMul: 2.0, Icon: "plate", Looks: []string{"torso_armour_plate"}, Material: "metal"},
	{ID: "leather", Slot: Chest, Name: "Leather Armor", ArmorMul: 1.3, Icon: "leather", Looks: []string{"torso_armour_leather"}, Material: "cloth"},
	{ID: "robe", Slot: Chest, Name: "Robe", ArmorMul: 0.7, Icon: "robe", Looks: []string{"torso_clothes_robe"}, Material: "cloth"},
	{ID: "greaves", Slot: Legs, Name: "Greaves", ArmorMul: 1.2, Icon: "greaves", Looks: []string{"legs_armour"}, Material: "metal"},
	{ID: "pants", Slot: Legs, Name: "Pants", ArmorMul: 0.6, Icon: "pants", Looks: []string{"legs_pants", "legs_pantaloons"}, Material: "cloth"},
	{ID: "boots", Slot: Feet, Name: "Boots", ArmorMul: 0.8, Icon: "boots", Looks: []string{"feet_boots_basic", "feet_boots_fold"}, Material: "cloth"},
	{ID: "sabatons", Slot: Feet, Name: "Sabatons", ArmorMul: 1.1, Icon: "boots", Looks: []string{"feet_armour"}, Material: "metal"},
	{ID: "gauntlets", Slot: Hands, Name: "Gauntlets", ArmorMul: 0.9, Icon: "gloves", Looks: []string{"arms_armour"}, Material: "metal"},
	{ID: "gloves", Slot: Hands, Name: "Gloves", ArmorMul: 0.5, Icon: "gloves", Looks: []string{"arms_gloves"}, Material: "cloth"},
	{ID: "ring", Slot: Ring, Name: "Ring", Icon: "ring"},
	{ID: "amulet", Slot: Amulet, Name: "Amulet", Icon: "amulet"},
}

var baseByID = func() map[string]Base {
	m := map[string]Base{}
	for _, b := range Bases {
		m[b.ID] = b
	}
	return m
}()

func BaseByID(id string) (Base, bool) { b, ok := baseByID[id]; return b, ok }

// Stat names used by affixes and equipment bonuses.
const (
	StatStr       = "str"
	StatAgi       = "agi"
	StatInt       = "int"
	StatVit       = "vit"
	StatCrit      = "crit_pct"
	StatDamage    = "damage"
	StatMagic     = "magic"
	StatArmor     = "armor"
	StatHP        = "hp"
	StatSpeed     = "speed_pct"
	StatLifeSteal = "lifesteal_pct"
	StatXP        = "xp_pct"
	StatMagicFind = "magic_find_pct"
)

type affixDef struct {
	stat           string
	prefix, suffix string
	min, perLvl    float64
	cap            float64 // 0 = no cap
	weight         float64
}

var affixDefs = []affixDef{
	{StatStr, "Brutal", "of the Bear", 1, 0.35, 0, 10},
	{StatAgi, "Nimble", "of the Fox", 1, 0.35, 0, 10},
	{StatInt, "Arcane", "of the Owl", 1, 0.35, 0, 10},
	{StatVit, "Sturdy", "of the Oak", 1, 0.35, 0, 10},
	{StatCrit, "Keen", "of Precision", 1, 0.12, 25, 6},
	{StatDamage, "Cruel", "of Slaying", 2, 0.8, 0, 7},
	{StatMagic, "Mystic", "of Sorcery", 2, 0.8, 0, 6},
	{StatArmor, "Warded", "of the Wall", 2, 0.7, 0, 7},
	{StatHP, "Vital", "of Life", 8, 3.5, 0, 8},
	{StatSpeed, "Swift", "of the Wind", 2, 0.12, 20, 3},
	{StatLifeSteal, "Vampiric", "of the Leech", 1, 0.06, 12, 2},
	{StatXP, "Wise", "of Learning", 2, 0.15, 25, 3},
	{StatMagicFind, "Lucky", "of Fortune", 3, 0.25, 40, 3},
}

// Affix is one rolled bonus.
type Affix struct {
	Stat   string  `json:"stat"`
	Value  float64 `json:"value"`
	Prefix string  `json:"prefix,omitempty"`
	Suffix string  `json:"suffix,omitempty"`
}

// Appearance is how the item shows up on the LPC character sheet.
type Appearance struct {
	Item    string   `json:"item"`
	Variant string   `json:"variant,omitempty"`
	Colors  []string `json:"colors,omitempty"`
}

// Item is the immutable generated part of an item.
type Item struct {
	RulesVersion int         `json:"rules_version"`
	Seed         uint64      `json:"seed,string"`
	Base         string      `json:"base"`
	Slot         Slot        `json:"slot"`
	Kind         string      `json:"kind,omitempty"`
	Rarity       Rarity      `json:"rarity"`
	ItemLevel    int         `json:"item_level"`
	Name         string      `json:"name"`
	DamageMin    int         `json:"damage_min,omitempty"`
	DamageMax    int         `json:"damage_max,omitempty"`
	Armor        int         `json:"armor,omitempty"`
	Cooldown     float64     `json:"cooldown,omitempty"`
	Range        float64     `json:"range,omitempty"`
	CritPct      float64     `json:"crit_pct,omitempty"`
	Affixes      []Affix     `json:"affixes"`
	Trait        string      `json:"trait,omitempty"` // mythic-only unique trait text
	IconSeed     uint64      `json:"icon_seed,string"`
	Icon         string      `json:"icon"`
	Hue          float64     `json:"hue"`
	Appearance   *Appearance `json:"appearance,omitempty"`
}

var metalByRarity = []string{"iron", "steel", "silver", "gold", "gold", "brass"}
var clothColors = []string{"brown", "leather", "walnut", "tan", "maroon", "red", "blue", "navy", "teal", "forest", "green", "gray", "black", "charcoal", "purple", "white"}

// RollRarity picks a rarity; luck (0..1+) shifts weight to better tiers.
func RollRarity(r *seed.Rand, luck float64, min Rarity) Rarity {
	w := []float64{60, 26, 10, 3.2, 0.7, 0.1}
	for i := 1; i < len(w); i++ {
		w[i] *= 1 + luck*float64(i)
	}
	for i := 0; i < int(min); i++ {
		w[i] = 0
	}
	return Rarity(r.Weighted(w))
}

// Generate creates an item. minRarity lets bosses guarantee quality.
func Generate(s uint64, itemLevel int, luck float64, minRarity Rarity) Item {
	if itemLevel < 1 {
		itemLevel = 1
	}
	r := seed.New(seed.Derive(s, "item"))
	base := seed.Pick(r, Bases)
	rar := RollRarity(r, luck, minRarity)
	it := Item{
		RulesVersion: RulesVersion,
		Seed:         s,
		Base:         base.ID,
		Slot:         base.Slot,
		Kind:         base.Kind,
		Rarity:       rar,
		ItemLevel:    itemLevel,
		Cooldown:     base.Cooldown,
		Range:        base.Range,
		CritPct:      base.CritPct,
		IconSeed:     r.Uint64(),
		Icon:         base.Icon,
		Hue:          r.Float(),
	}
	L := float64(itemLevel)
	quality := 1 + 0.12*float64(rar) + r.FRange(-0.08, 0.08)
	if base.DmgMul > 0 {
		avg := (4 + 2.1*L) * base.DmgMul * quality
		spread := 0.2 + r.FRange(0, 0.15)
		it.DamageMin = int(math.Max(1, math.Round(avg*(1-spread))))
		it.DamageMax = int(math.Max(float64(it.DamageMin+1), math.Round(avg*(1+spread))))
	}
	if base.ArmorMul > 0 {
		it.Armor = int(math.Max(1, math.Round((2+1.1*L)*base.ArmorMul*quality)))
	}
	// Affixes: no stat twice.
	pool := append([]affixDef(nil), affixDefs...)
	for i := 0; i < rar.AffixCount() && len(pool) > 0; i++ {
		w := make([]float64, len(pool))
		for j, a := range pool {
			w[j] = a.weight
		}
		k := r.Weighted(w)
		a := pool[k]
		pool = append(pool[:k], pool[k+1:]...)
		v := (a.min + a.perLvl*L) * r.FRange(0.7, 1.15) * (1 + 0.08*float64(rar))
		if a.cap > 0 && v > a.cap {
			v = a.cap
		}
		it.Affixes = append(it.Affixes, Affix{Stat: a.stat, Value: math.Round(v*10) / 10, Prefix: a.prefix, Suffix: a.suffix})
	}
	it.Name = itemName(r, base, rar, it.Affixes)
	if rar == Mythic {
		it.Trait = seed.Pick(r, mythicTraits)
	}
	if len(base.Looks) > 0 {
		ap := &Appearance{Item: base.Looks[r.Intn(len(base.Looks))]}
		switch base.Material {
		case "metal":
			ap.Variant = metalByRarity[rar]
			ap.Colors = []string{metalByRarity[rar]}
		case "cloth":
			ap.Colors = []string{seed.Pick(r, clothColors)}
		}
		it.Appearance = ap
	}
	return it
}

var mythicTraits = []string{
	"Echoing Strike: hits sometimes repeat",
	"Starlit: glows under the night sky",
	"Worldbound: remembers every place it has been",
	"Soulbound: grows stronger with its first owner",
	"Voidtouched: bends light around it",
}

func itemName(r *seed.Rand, b Base, rar Rarity, affixes []Affix) string {
	switch {
	case rar >= Legendary:
		// Legendary and mythic items get their own generated names.
		return fmt.Sprintf("%s, %s %s", names.Word(r, r.Range(2, 3)), seed.Pick(r, legendTitles), b.Name)
	case len(affixes) == 0:
		return b.Name
	case len(affixes) == 1:
		if r.Chance(0.5) {
			return affixes[0].Prefix + " " + b.Name
		}
		return b.Name + " " + affixes[0].Suffix
	default:
		return affixes[0].Prefix + " " + b.Name + " " + affixes[1].Suffix
	}
}

var legendTitles = []string{"the Unbroken", "the Last", "Dawnbringer's", "the Hollow King's", "the Wanderer's", "the Eternal", "the Sunken", "the Starfallen"}

// ----- mutable progression of an owned item -----

// State is the mutable part of an owned item.
type State struct {
	Enhance int   `json:"enhance"`
	Level   int   `json:"level"`
	XP      int64 `json:"xp"`
}

// XPToNext for item leveling.
func XPToNext(level int) int64 { return int64(40 * math.Pow(float64(level), 1.5)) }

// AddXP applies XP respecting the rarity cap; returns levels gained.
func AddXP(it Item, st *State, xp int64) int {
	if st.Level < 1 {
		st.Level = 1
	}
	gained := 0
	st.XP += xp
	for st.Level < it.Rarity.MaxLevel() && st.XP >= XPToNext(st.Level) {
		st.XP -= XPToNext(st.Level)
		st.Level++
		gained++
	}
	if st.Level >= it.Rarity.MaxLevel() {
		st.XP = 0
	}
	return gained
}

// Scale is the stat multiplier from enhancement and item level.
func Scale(st State) float64 {
	lvl := st.Level
	if lvl < 1 {
		lvl = 1
	}
	return 1 + 0.07*float64(st.Enhance) + 0.035*float64(lvl-1)
}

// Effective returns the item's current bonuses as stat -> value.
func Effective(it Item, st State) map[string]float64 {
	m := Scale(st)
	out := map[string]float64{}
	if it.DamageMax > 0 {
		out[StatDamage] += math.Round(float64(it.DamageMin+it.DamageMax) / 2 * m)
	}
	if it.Armor > 0 {
		out[StatArmor] += math.Round(float64(it.Armor) * m)
	}
	if it.CritPct > 0 {
		out[StatCrit] += it.CritPct
	}
	for _, a := range it.Affixes {
		v := a.Value
		switch a.Stat {
		case StatCrit, StatSpeed, StatLifeSteal, StatXP, StatMagicFind:
			v *= 1 + (m-1)*0.4 // percentages scale gently
		default:
			v *= m
		}
		out[a.Stat] += math.Round(v*10) / 10
	}
	return out
}

// ----- enhancement -----

var enhanceChance = []float64{1, 1, 0.95, 0.9, 0.8, 0.7, 0.6, 0.5, 0.42, 0.35, 0.3, 0.25, 0.2, 0.16, 0.13, 0.1, 0.08, 0.06, 0.05, 0.04}

// EnhanceChance is the success probability of going from cur to cur+1.
func EnhanceChance(cur int, bonus float64) float64 {
	if cur < 0 || cur >= len(enhanceChance) {
		return 0
	}
	return math.Min(1, enhanceChance[cur]+bonus)
}

// EnhanceCost in gold and essence for going from cur to cur+1.
func EnhanceCost(it Item, cur int) (gold, essence int64) {
	n := float64(cur + 1)
	v := it.Rarity.Value()
	gold = int64(math.Round(40 * n * n * v * (1 + float64(it.ItemLevel)/20)))
	essence = int64(math.Ceil(n * v))
	return
}

// EnhanceResult of one attempt.
type EnhanceResult struct {
	Success bool    `json:"success"`
	From    int     `json:"from"`
	To      int     `json:"to"`
	Chance  float64 `json:"chance"`
	Roll    float64 `json:"roll"`
}

// Enhance performs attempt #attempt on an item. The roll is derived from a
// server secret so it is reproducible for audits but unpredictable to clients.
func Enhance(serverSecret uint64, itemID string, attempt int64, it Item, st State, bonus float64) (EnhanceResult, error) {
	if st.Enhance >= it.Rarity.MaxEnhance() {
		return EnhanceResult{}, fmt.Errorf("item is already at max enhancement +%d", st.Enhance)
	}
	chance := EnhanceChance(st.Enhance, bonus)
	roll := seed.New(seed.Derive(serverSecret, "enhance", itemID, attempt)).Float()
	res := EnhanceResult{From: st.Enhance, Chance: chance, Roll: roll}
	if roll < chance {
		res.Success, res.To = true, st.Enhance+1
	} else if st.Enhance >= 7 {
		res.To = st.Enhance - 1 // high-level failures lose a level, never the item
	} else {
		res.To = st.Enhance
	}
	return res, nil
}

// SalvageValue returns the essence and gold from destroying an item.
func SalvageValue(it Item, st State) (essence, gold int64) {
	v := it.Rarity.Value()
	essence = int64(math.Round(v*(1+float64(it.ItemLevel)/6))) + int64(st.Enhance*2)
	gold = int64(math.Round(v * float64(it.ItemLevel) * 3))
	return
}

// DisplayName includes the enhancement, e.g. "+3 Keen Sword of the Fox".
func DisplayName(it Item, st State) string {
	if st.Enhance > 0 {
		return fmt.Sprintf("+%d %s", st.Enhance, it.Name)
	}
	return it.Name
}

// Summary is a short human description (used by logs and history).
func Summary(it Item) string {
	var parts []string
	for _, a := range it.Affixes {
		parts = append(parts, fmt.Sprintf("%s %+g", a.Stat, a.Value))
	}
	return fmt.Sprintf("%s [%s ilvl %d] %s", it.Name, it.Rarity, it.ItemLevel, strings.Join(parts, ", "))
}
