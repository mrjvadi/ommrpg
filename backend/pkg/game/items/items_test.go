package items

import (
	"encoding/json"
	"testing"
)

func TestGenerateDeterministic(t *testing.T) {
	a, b := Generate(1234, 10, 0, Common), Generate(1234, 10, 0, Common)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("not deterministic")
	}
	var back Item
	if err := json.Unmarshal(ja, &back); err != nil || back.Seed != 1234 || back.Rarity != a.Rarity {
		t.Fatalf("round trip failed: %v", err)
	}
}

func TestRarityDistributionAndAffixes(t *testing.T) {
	counts := map[Rarity]int{}
	for s := uint64(0); s < 20000; s++ {
		it := Generate(s, 20, 0, Common)
		counts[it.Rarity]++
		if len(it.Affixes) != it.Rarity.AffixCount() {
			t.Fatalf("affix count mismatch %v", it)
		}
		seen := map[string]bool{}
		for _, a := range it.Affixes {
			if seen[a.Stat] {
				t.Fatal("duplicate affix stat")
			}
			seen[a.Stat] = true
		}
		if it.Slot == Weapon && (it.DamageMin <= 0 || it.DamageMax <= it.DamageMin) {
			t.Fatalf("bad weapon damage %+v", it)
		}
	}
	if counts[Common] < counts[Rare] || counts[Legendary] == 0 {
		t.Fatalf("unexpected distribution %v", counts)
	}
	if Generate(5, 10, 0, Epic).Rarity < Epic {
		t.Fatal("min rarity not honoured")
	}
}

func TestLevelAndEnhance(t *testing.T) {
	it := Generate(99, 5, 0, Common)
	st := State{Level: 1}
	gained := AddXP(it, &st, 1_000_000)
	if st.Level != it.Rarity.MaxLevel() || gained == 0 {
		t.Fatalf("level cap not applied: %+v", st)
	}
	before := Effective(it, State{Level: 1})
	after := Effective(it, State{Level: 1, Enhance: 5})
	for k, v := range before {
		if after[k] < v {
			t.Fatalf("enhance reduced %s", k)
		}
	}
	res, err := Enhance(42, "item-1", 1, it, State{Enhance: 0}, 0)
	if err != nil || !res.Success || res.To != 1 {
		t.Fatalf("+0 -> +1 must always succeed: %+v %v", res, err)
	}
	if _, err := Enhance(42, "x", 1, it, State{Enhance: it.Rarity.MaxEnhance()}, 0); err == nil {
		t.Fatal("expected max enhance error")
	}
	// failures at +7 or more drop one level
	fails := 0
	for a := int64(0); a < 200; a++ {
		r, _ := Enhance(42, "item-2", a, Generate(1, 10, 0, Legendary), State{Enhance: 9}, 0)
		if !r.Success {
			fails++
			if r.To != 8 {
				t.Fatal("expected downgrade on failure")
			}
		}
	}
	if fails == 0 {
		t.Fatal("expected some failures at +9")
	}
}

func TestLoot(t *testing.T) {
	l := RollLoot(7, 10, "boss", 0)
	if len(l.Items) < 3 || l.Gold <= 0 {
		t.Fatalf("boss loot too small %+v", l)
	}
	for _, d := range l.Items {
		if Generate(d.Seed, d.ItemLevel, d.Luck, d.MinRarity).Rarity < Rare {
			t.Fatal("boss drop below rare")
		}
	}
}
