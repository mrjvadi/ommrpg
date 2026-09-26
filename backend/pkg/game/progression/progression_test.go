package progression

import "testing"

func TestXPCurveIncreasing(t *testing.T) {
	prev := int64(0)
	for l := 1; l < MaxLevel; l++ {
		x := XPToNext(l)
		if x <= prev {
			t.Fatalf("curve not increasing at %d", l)
		}
		prev = x
	}
	if XPToNext(MaxLevel) != 0 {
		t.Fatal("max level must need 0")
	}
}

func TestDeriveFistsAndWeapon(t *testing.T) {
	base := Attributes{5, 5, 5, 5}
	h := Hidden{Root: Mundane, Talent: NoTalent, Potential: 1}
	fists := Derive(1, base, h, Bonuses{})
	if fists.WeaponKind != "melee" || fists.Attack <= 0 || fists.MaxHP <= 0 {
		t.Fatalf("bad fists %+v", fists)
	}
	bow := Derive(1, base, h, Bonuses{WeaponKind: "ranged", Damage: 10, Range: 6, Cooldown: 0.8})
	if bow.Range != 6 || bow.Attack != 20 {
		t.Fatalf("bad bow %+v", bow)
	}
}

func TestAwaken(t *testing.T) {
	h := Hidden{Root: Mundane}
	if c := Awaken(Attributes{Str: 10}, h, Behaviour{MeleeKills: 50}); c != Warrior {
		t.Fatal(c)
	}
	if c := Awaken(Attributes{Int: 10}, Hidden{Root: Voidborn}, Behaviour{MagicKills: 50}); c != VoidWalker {
		t.Fatal(c)
	}
	if c := Awaken(Attributes{}, h, Behaviour{MeleeKills: 5, Upgrades: 10}); c != Artisan {
		t.Fatal(c)
	}
}

func TestHiddenDeterministic(t *testing.T) {
	if RollHidden(9) != RollHidden(9) {
		t.Fatal("not deterministic")
	}
}
