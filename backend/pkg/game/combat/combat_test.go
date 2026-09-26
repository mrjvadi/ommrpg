package combat

import "testing"

func TestElementalWeakness(t *testing.T) {
	plain := ResolveElemental(1, 100, 10, 0, "", 0, 1)
	weak := ResolveElemental(1, 100, 10, 0, "fire", 20, 1.6)
	resist := ResolveElemental(1, 100, 10, 0, "fire", 20, 0.2)
	if weak.Damage <= resist.Damage || resist.Damage <= plain.Damage || weak.Element != "fire" {
		t.Fatalf("plain %d weak %d resist %d", plain.Damage, weak.Damage, resist.Damage)
	}
}
