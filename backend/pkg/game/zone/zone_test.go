package zone

import "testing"

func TestRoundTrip(t *testing.T) {
	for _, z := range []ID{World(3), DungeonFloor("abc123", 2)} {
		p, err := Parse(z.String())
		if err != nil || p != z {
			t.Fatalf("%v -> %v %v", z, p, err)
		}
	}
	for _, bad := range []string{"", "x:1", "w:a", "d::1", "d:a"} {
		if _, err := Parse(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
