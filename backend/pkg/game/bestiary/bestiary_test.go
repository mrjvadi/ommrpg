package bestiary

import (
	"testing"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
)

func TestSpeciesDeterministicAndCovered(t *testing.T) {
	a := ForWorld(5)
	cache.Delete(uint64(5))
	b := ForWorld(5)
	if len(a) != len(families)*PerFamily {
		t.Fatalf("len=%d", len(a))
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].SpriteSeed != b[i].SpriteSeed {
			t.Fatal("species not deterministic")
		}
	}
	for _, bm := range []world.Biome{world.BPlains, world.BForest, world.BDesert, world.BDungeon} {
		if len(InBiome(5, bm)) == 0 {
			t.Fatalf("no species for %v", bm)
		}
	}
}

func TestChunkSpawnsWalkableAndOutsideSafeZone(t *testing.T) {
	p := world.Params{ID: 1, Seed: 77, SizeChunks: 32}
	total := 0
	for cy := 0; cy < 32; cy++ {
		for cx := 0; cx < 32; cx++ {
			for _, s := range ChunkSpawns(p, cx, cy) {
				total++
				if !p.Walkable(s.X, s.Y) {
					t.Fatalf("spawn %s on blocked tile", s.ID)
				}
				if p.DistToSpawn(s.X, s.Y) < world.SafeRadius {
					t.Fatalf("spawn %s inside safe zone", s.ID)
				}
			}
		}
	}
	if total < 500 {
		t.Fatalf("too few monsters: %d", total)
	}
}
