package dungeon

import (
	"testing"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/bestiary"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
)

func TestFloorsValid(t *testing.T) {
	for s := uint64(1); s < 60; s++ {
		spec := Spec{Seed: s, WorldSeed: 3, Tier: int(s%8) + 1}
		for fl := 0; fl < spec.Floors(); fl++ {
			f := Generate(spec, fl, "inst")
			if !f.Reachable() {
				t.Fatalf("seed %d floor %d has unreachable tiles", s, fl)
			}
			if !f.Walkable(f.Start.X, f.Start.Y) {
				t.Fatal("start not walkable")
			}
			if _, o := f.Tile(f.Exit.X, f.Exit.Y); o != world.ODungeonExit {
				t.Fatal("missing exit")
			}
			last := fl == spec.Floors()-1
			if last != (f.BossID != "") || last == (f.Stairs != nil) {
				t.Fatalf("stairs/boss wrong on floor %d/%d", fl, spec.Floors())
			}
			for _, m := range f.Monsters {
				if !f.Walkable(m.X, m.Y) && m.Rank != bestiary.Boss {
					t.Fatalf("monster on blocked tile")
				}
			}
		}
	}
}

func TestDeterministic(t *testing.T) {
	a := Generate(Spec{Seed: 9, WorldSeed: 1, Tier: 3}, 1, "i")
	b := Generate(Spec{Seed: 9, WorldSeed: 1, Tier: 3}, 1, "i")
	if string(a.Ground) != string(b.Ground) || len(a.Monsters) != len(b.Monsters) {
		t.Fatal("not deterministic")
	}
}
