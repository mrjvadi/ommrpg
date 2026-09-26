// Package dungeon generates dungeon floors (rooms, corridors, monsters,
// chests, stairs and boss) from a seed. A dungeon instance only stores its
// seed; every floor is regenerated on demand by any service.
package dungeon

import (
	"fmt"
	"sort"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/bestiary"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Room struct {
	X, Y, W, H int
}

func (r Room) Center() Point { return Point{r.X + r.W/2, r.Y + r.H/2} }

func (r Room) overlaps(o Room, pad int) bool {
	return r.X-pad < o.X+o.W && r.X+r.W+pad > o.X && r.Y-pad < o.Y+o.H && r.Y+r.H+pad > o.Y
}

// Chest placed in a room; opened once per instance.
type Chest struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
}

// Spec identifies a dungeon; together with Floor it identifies a floor.
type Spec struct {
	Seed      uint64 `json:"seed,string"`
	WorldSeed uint64 `json:"world_seed,string"`
	Tier      int    `json:"tier"`
	Name      string `json:"name"`
}

// Floors returns the number of floors for a tier.
func (s Spec) Floors() int {
	f := 1 + s.Tier/2
	if f > 5 {
		f = 5
	}
	return f
}

// Floor is a generated dungeon floor.
type Floor struct {
	Index    int              `json:"floor"`
	Floors   int              `json:"floors"`
	W        int              `json:"w"`
	H        int              `json:"h"`
	Ground   []byte           `json:"ground"`
	Objects  []byte           `json:"objects"`
	Start    Point            `json:"start"`
	Exit     Point            `json:"exit"`
	Stairs   *Point           `json:"stairs,omitempty"`
	Monsters []bestiary.Spawn `json:"monsters"`
	Chests   []Chest          `json:"chests"`
	BossID   string           `json:"boss_id,omitempty"`
	rooms    []Room
}

func (f *Floor) idx(x, y int) int { return y*f.W + x }

func (f *Floor) In(x, y int) bool { return x >= 0 && y >= 0 && x < f.W && y < f.H }

// Tile returns the ground/object at a tile of the floor.
func (f *Floor) Tile(x, y int) (world.Ground, world.Object) {
	if !f.In(x, y) {
		return world.GDungeonWall, world.ONone
	}
	i := f.idx(x, y)
	return world.Ground(f.Ground[i]), world.Object(f.Objects[i])
}

func (f *Floor) Walkable(x, y int) bool {
	g, o := f.Tile(x, y)
	return g.Walkable() && !o.Blocking()
}

// Level of monsters on a floor.
func (s Spec) Level(floor int) int { return s.Tier*6 + floor*2 }

// Generate builds floor `floor` (0-based) of the dungeon.
func Generate(s Spec, floor int, instanceID string) *Floor {
	r := seed.New(seed.Derive(s.Seed, "floor", floor))
	size := 40 + 6*s.Tier
	if size > 72 {
		size = 72
	}
	f := &Floor{Index: floor, Floors: s.Floors(), W: size, H: size,
		Ground:  make([]byte, size*size),
		Objects: make([]byte, size*size),
	}
	for i := range f.Ground {
		f.Ground[i] = byte(world.GDungeonWall)
	}
	// rooms
	want := 7 + s.Tier + r.Intn(4)
	for tries := 0; tries < 400 && len(f.rooms) < want; tries++ {
		rm := Room{W: r.Range(5, 10), H: r.Range(5, 9)}
		rm.X, rm.Y = r.Range(1, size-rm.W-2), r.Range(1, size-rm.H-2)
		ok := true
		for _, o := range f.rooms {
			if rm.overlaps(o, 2) {
				ok = false
				break
			}
		}
		if ok {
			f.rooms = append(f.rooms, rm)
		}
	}
	for _, rm := range f.rooms {
		for y := rm.Y; y < rm.Y+rm.H; y++ {
			for x := rm.X; x < rm.X+rm.W; x++ {
				f.Ground[f.idx(x, y)] = byte(world.GDungeonFloor)
			}
		}
	}
	// connect rooms with a minimum spanning tree (Prim) + a few loops
	connected := map[int]bool{0: true}
	for len(connected) < len(f.rooms) {
		bestA, bestB, bestD := -1, -1, 1<<30
		for a := range connected {
			for b := range f.rooms {
				if connected[b] {
					continue
				}
				ca, cb := f.rooms[a].Center(), f.rooms[b].Center()
				d := abs(ca.X-cb.X) + abs(ca.Y-cb.Y)
				if d < bestD || (d == bestD && (a < bestA || (a == bestA && b < bestB))) {
					bestA, bestB, bestD = a, b, d
				}
			}
		}
		f.corridor(r, f.rooms[bestA].Center(), f.rooms[bestB].Center())
		connected[bestB] = true
	}
	for i := 0; i < 2 && len(f.rooms) > 3; i++ {
		a, b := r.Intn(len(f.rooms)), r.Intn(len(f.rooms))
		if a != b {
			f.corridor(r, f.rooms[a].Center(), f.rooms[b].Center())
		}
	}
	// start in room 0, far room = boss/stairs (by BFS distance)
	start := f.rooms[0].Center()
	f.Start = start
	f.Exit = Point{start.X, start.Y - 1}
	if !f.Walkable(f.Exit.X, f.Exit.Y) {
		f.Exit = Point{start.X - 1, start.Y}
	}
	dist := f.bfs(start)
	order := make([]int, len(f.rooms))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		ca, cb := f.rooms[order[a]].Center(), f.rooms[order[b]].Center()
		return dist[f.idx(ca.X, ca.Y)] > dist[f.idx(cb.X, cb.Y)]
	})
	far := f.rooms[order[0]]
	last := floor == f.Floors-1
	if floor == 0 {
		f.Objects[f.idx(f.Exit.X, f.Exit.Y)] = byte(world.ODungeonExit)
	} else {
		// upper floors: the exit tile still leads out (return-bound to entrance)
		f.Objects[f.idx(f.Exit.X, f.Exit.Y)] = byte(world.ODungeonExit)
	}
	if !last {
		c := far.Center()
		f.Stairs = &Point{c.X, c.Y}
		f.Objects[f.idx(c.X, c.Y)] = byte(world.OStairsDown)
	}
	// torches and bones for mood
	for _, rm := range f.rooms {
		if r.Chance(0.7) {
			x := rm.X + r.Intn(rm.W)
			if rm.Y-1 >= 0 && f.Ground[f.idx(x, rm.Y-1)] == byte(world.GDungeonWall) {
				f.Objects[f.idx(x, rm.Y-1)] = byte(world.OTorch)
			}
		}
		if r.Chance(0.4) {
			p := Point{rm.X + r.Intn(rm.W), rm.Y + r.Intn(rm.H)}
			if f.Objects[f.idx(p.X, p.Y)] == 0 && p != start {
				f.Objects[f.idx(p.X, p.Y)] = byte(world.OBones)
			}
		}
	}
	// monsters
	cands := bestiary.InBiome(s.WorldSeed, world.BDungeon)
	lvl := s.Level(floor)
	n := 0
	for ri, rm := range f.rooms {
		if ri == 0 {
			continue // start room is safe
		}
		count := r.Range(1, 2+s.Tier/2)
		for k := 0; k < count; k++ {
			p := Point{rm.X + 1 + r.Intn(max(1, rm.W-2)), rm.Y + 1 + r.Intn(max(1, rm.H-2))}
			if !f.Walkable(p.X, p.Y) || f.Objects[f.idx(p.X, p.Y)] != 0 {
				continue
			}
			rank := bestiary.Normal
			if r.Chance(0.12) {
				rank = bestiary.Elite
			}
			f.Monsters = append(f.Monsters, bestiary.Spawn{
				ID: fmt.Sprintf("%s_f%d_m%d", instanceID, floor, n), Species: seed.Pick(r, cands).ID,
				Level: lvl + r.Range(0, 1), Rank: rank, X: p.X, Y: p.Y,
			})
			n++
		}
	}
	if last {
		c := far.Center()
		boss := bestiary.Spawn{ID: fmt.Sprintf("%s_f%d_boss", instanceID, floor), Species: seed.Pick(r, cands).ID,
			Level: lvl + 2, Rank: bestiary.Boss, X: c.X, Y: c.Y}
		f.Monsters = append(f.Monsters, boss)
		f.BossID = boss.ID
	}
	// chests in 1-2 non-start rooms
	for c := 0; c < 1+r.Intn(2) && len(f.rooms) > 2; c++ {
		rm := f.rooms[1+r.Intn(len(f.rooms)-1)]
		p := Point{rm.X + r.Intn(rm.W), rm.Y}
		if f.Objects[f.idx(p.X, p.Y)] != 0 {
			continue
		}
		f.Objects[f.idx(p.X, p.Y)] = byte(world.OChest)
		f.Chests = append(f.Chests, Chest{ID: fmt.Sprintf("%s_f%d_c%d", instanceID, floor, c), X: p.X, Y: p.Y})
	}
	return f
}

func (f *Floor) corridor(r *seed.Rand, a, b Point) {
	x, y := a.X, a.Y
	horizFirst := r.Chance(0.5)
	carve := func() { f.Ground[f.idx(x, y)] = byte(world.GDungeonFloor) }
	step := func(tx, ty int) {
		for x != tx {
			x += sign(tx - x)
			carve()
		}
		for y != ty {
			y += sign(ty - y)
			carve()
		}
	}
	if horizFirst {
		step(b.X, a.Y)
		step(b.X, b.Y)
	} else {
		step(a.X, b.Y)
		step(b.X, b.Y)
	}
}

func (f *Floor) bfs(from Point) []int {
	dist := make([]int, f.W*f.H)
	for i := range dist {
		dist[i] = -1
	}
	q := []Point{from}
	dist[f.idx(from.X, from.Y)] = 0
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := p.X+d[0], p.Y+d[1]
			if !f.In(nx, ny) || dist[f.idx(nx, ny)] >= 0 || f.Ground[f.idx(nx, ny)] != byte(world.GDungeonFloor) {
				continue
			}
			dist[f.idx(nx, ny)] = dist[f.idx(p.X, p.Y)] + 1
			q = append(q, Point{nx, ny})
		}
	}
	return dist
}

// Reachable reports whether every floor tile is reachable from the start.
func (f *Floor) Reachable() bool {
	d := f.bfs(f.Start)
	for i, g := range f.Ground {
		if g == byte(world.GDungeonFloor) && d[i] < 0 {
			return false
		}
	}
	return true
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func sign(a int) int {
	switch {
	case a > 0:
		return 1
	case a < 0:
		return -1
	}
	return 0
}
