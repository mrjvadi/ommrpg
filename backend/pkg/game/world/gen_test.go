package world

import (
	"testing"
	"time"
)

func testParams() Params { return Params{ID: 1, Seed: 123456789, SizeChunks: 64} }

func TestSpawnIsWalkablePlaza(t *testing.T) {
	p := testParams()
	x, y := p.Spawn()
	g, o := p.Tile(x, y)
	if g != GPath || o != OShrine {
		t.Fatalf("spawn tile = %v/%v", g, o)
	}
	if !p.Walkable(x+1, y) || !p.Walkable(x, y+1) {
		t.Fatal("tiles next to shrine must be walkable")
	}
}

func TestChunkDeterministic(t *testing.T) {
	p := testParams()
	a, b := p.ChunkTerrain(10, 12), p.ChunkTerrain(10, 12)
	for i := range a.Ground {
		if a.Ground[i] != b.Ground[i] || a.Objects[i] != b.Objects[i] {
			t.Fatal("chunk not deterministic")
		}
	}
	// Tile() and ChunkTerrain() must agree.
	for i := 0; i < ChunkSize*ChunkSize; i += 37 {
		x, y := 10*ChunkSize+i%ChunkSize, 12*ChunkSize+i/ChunkSize
		g, o := p.Tile(x, y)
		if byte(g) != a.Ground[i] || byte(o) != a.Objects[i] {
			t.Fatal("Tile and ChunkTerrain disagree")
		}
	}
}

func TestBiomeVarietyAndEntrances(t *testing.T) {
	p := testParams()
	seen := map[Biome]int{}
	for y := 0; y < p.SizeTiles(); y += 8 {
		for x := 0; x < p.SizeTiles(); x += 8 {
			seen[p.BiomeAt(x, y)]++
		}
	}
	if len(seen) < 6 {
		t.Fatalf("expected varied biomes, got %v", seen)
	}
	land := 0
	for b, n := range seen {
		if b != BOcean {
			land += n
		}
	}
	if land < len(seen)*10 {
		t.Fatalf("too little land: %v", seen)
	}
	entrances := 0
	for cy := 0; cy < p.SizeChunks; cy++ {
		for cx := 0; cx < p.SizeChunks; cx++ {
			if e, ok := p.EntranceIn(cx, cy); ok {
				entrances++
				if g, o := p.Tile(e.X, e.Y); !g.Walkable() || o != ODungeonEntrance {
					t.Fatalf("bad entrance tile %v %v", g, o)
				}
			}
		}
	}
	if entrances < 20 {
		t.Fatalf("too few dungeon entrances: %d", entrances)
	}
	t.Logf("biomes=%v entrances=%d", seen, entrances)
}

func TestChunkOfNegative(t *testing.T) {
	if cx, cy := ChunkOf(-1, 31); cx != -1 || cy != 0 {
		t.Fatalf("got %d,%d", cx, cy)
	}
}

func BenchmarkChunk(b *testing.B) {
	p := testParams()
	for i := 0; i < b.N; i++ {
		p.ChunkTerrain(i%64, (i/64)%64)
	}
}

func TestRiversFlowToSea(t *testing.T) {
	p := Params{ID: 1, Seed: 555, SizeChunks: 64}
	start := time.Now()
	net := p.rivers()
	t.Logf("river network for %dx%d cells in %v", net.n, net.n, time.Since(start))
	rivers := 0
	for i, f := range net.flow {
		if f < riverThreshold {
			continue
		}
		rivers++
		// following parents always terminates (a tree rooted at the sea)
		steps, c := 0, int32(i)
		for c >= 0 && steps < len(net.flow) {
			c = net.parent[c]
			steps++
		}
		if steps >= len(net.flow) {
			t.Fatal("drainage cycle")
		}
	}
	if rivers < 50 {
		t.Fatalf("too few river cells: %d", rivers)
	}
	sx, sy := p.Spawn()
	if !p.Walkable(sx+1, sy) {
		t.Fatal("plaza broken by rivers")
	}
}
