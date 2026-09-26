package world

import (
	"container/heap"
	"math"
	"sync"

	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// Rivers are computed once per world on a coarse grid (riverCell tiles per
// cell) with two classic hydrology algorithms:
//
//  1. Priority-Flood (Barnes, Lehman & Mulla 2014): starting from the ocean,
//     cells are visited in order of (filled) elevation with a min-heap. Each
//     cell's drainage parent is the cell it was reached from, which yields a
//     depression-free drainage tree that always ends in the sea.
//  2. Flow accumulation: visiting cells in reverse flood order (peaks first),
//     every cell passes its accumulated rainfall to its parent.
//
// Cells whose accumulated flow exceeds a threshold are river cells; a river
// is drawn as segments from each river cell to its downstream parent, with a
// width that grows logarithmically with the flow. Rivers are shallow water:
// walkable, so they never cut the map into unreachable parts.
const (
	riverCell      = 4
	riverThreshold = 420.0
)

type riverNet struct {
	n      int     // cells per side
	parent []int32 // drainage parent (-1 = sea/sink)
	flow   []float32
}

var riverCache sync.Map // key -> *riverNet

type cellItem struct {
	idx  int32
	elev float32
}

type cellHeap []cellItem

func (h cellHeap) Len() int { return len(h) }
func (h cellHeap) Less(i, j int) bool {
	if h[i].elev == h[j].elev {
		return h[i].idx < h[j].idx // deterministic tie-break
	}
	return h[i].elev < h[j].elev
}
func (h cellHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *cellHeap) Push(x any)   { *h = append(*h, x.(cellItem)) }
func (h *cellHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}

func (p Params) rivers() *riverNet {
	key := p.Seed ^ uint64(p.SizeChunks)<<48
	if v, ok := riverCache.Load(key); ok {
		return v.(*riverNet)
	}
	n := p.SizeTiles() / riverCell
	total := n * n
	elev := make([]float32, total)
	rain := make([]float32, total)
	sea := make([]bool, total)
	for cy := 0; cy < n; cy++ {
		for cx := 0; cx < n; cx++ {
			s := p.SampleAt(cx*riverCell+riverCell/2, cy*riverCell+riverCell/2)
			i := cy*n + cx
			elev[i] = float32(s.Elevation)
			sea[i] = s.Biome == BOcean
			// wetter and colder (snow melt) places feed more water
			rain[i] = float32(0.4 + s.Moisture + math.Max(0, 0.45-s.Temperature))
		}
	}
	net := &riverNet{n: n, parent: make([]int32, total), flow: make([]float32, total)}
	for i := range net.parent {
		net.parent[i] = -2 // unvisited
	}
	h := &cellHeap{}
	for i := 0; i < total; i++ {
		cx, cy := i%n, i/n
		if sea[i] || cx == 0 || cy == 0 || cx == n-1 || cy == n-1 {
			net.parent[i] = -1
			heap.Push(h, cellItem{int32(i), elev[i]})
		}
	}
	order := make([]int32, 0, total)
	// small per-cell jitter breaks up perfectly straight channels on flats
	jit := func(i int) float32 { return float32(seed.New(seed.Derive(p.Seed, "river", i)).Float()) * 1e-4 }
	for h.Len() > 0 {
		c := heap.Pop(h).(cellItem)
		order = append(order, c.idx)
		cx, cy := int(c.idx)%n, int(c.idx)/n
		for _, d := range [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || ny < 0 || nx >= n || ny >= n {
				continue
			}
			j := ny*n + nx
			if net.parent[j] != -2 {
				continue
			}
			net.parent[j] = c.idx
			e := elev[j]
			if e <= c.elev {
				e = c.elev + 1e-5 + jit(j) // fill the depression
			}
			heap.Push(h, cellItem{int32(j), e})
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		c := order[i]
		if sea[c] {
			continue
		}
		net.flow[c] += rain[c]
		if par := net.parent[c]; par >= 0 && !sea[par] {
			net.flow[par] += net.flow[c]
		}
	}
	riverCache.Store(key, net)
	return net
}

// Warm precomputes the world's river network (about half a second for a
// 64x64-chunk world) so the first player request does not pay for it.
func (p Params) Warm() { p.rivers() }

// riverWidth returns the half-width (tiles) of the river segment nearest to
// tile (x,y), or 0 when the tile is not in a river.
func (p Params) riverAt(x, y int) float64 {
	net := p.rivers()
	n := net.n
	px, py := float64(x)+0.5, float64(y)+0.5
	cx, cy := x/riverCell, y/riverCell
	best := 0.0
	center := func(i int32) (float64, float64) {
		return float64(int(i)%n)*riverCell + riverCell/2, float64(int(i)/n)*riverCell + riverCell/2
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			ax, ay := cx+dx, cy+dy
			if ax < 0 || ay < 0 || ax >= n || ay >= n {
				continue
			}
			i := int32(ay*n + ax)
			f := float64(net.flow[i])
			par := net.parent[i]
			if f < riverThreshold || par < 0 {
				continue
			}
			w := 0.55 + 0.45*math.Log(f/riverThreshold+1)
			if w > 2.6 {
				w = 2.6
			}
			x1, y1 := center(i)
			x2, y2 := center(par)
			if segDist(px, py, x1, y1, x2, y2) <= w && w > best {
				best = w
			}
		}
	}
	return best
}

func segDist(px, py, x1, y1, x2, y2 float64) float64 {
	vx, vy := x2-x1, y2-y1
	l := vx*vx + vy*vy
	t := 0.0
	if l > 0 {
		t = math.Max(0, math.Min(1, ((px-x1)*vx+(py-y1)*vy)/l))
	}
	return math.Hypot(px-(x1+t*vx), py-(y1+t*vy))
}
