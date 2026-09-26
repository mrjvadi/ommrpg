// Package zones resolves zone ids into regenerated terrain and monsters for
// the realtime services (presence, combat, dungeon). World params and
// dungeon instances are fetched once over NATS; everything else is
// regenerated from seeds and cached in memory.
package zones

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/bestiary"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/dungeon"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/zone"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
)

type Resolver struct {
	bus       *bus.Bus
	worlds    *lru.Cache[int, c.WorldInfo]
	instances *lru.Cache[string, c.Instance]
	floors    *lru.Cache[string, *dungeon.Floor]
	spawns    *lru.Cache[string, []bestiary.Spawn]
}

func New(b *bus.Bus) *Resolver {
	return &Resolver{
		bus:       b,
		worlds:    lru.New[int, c.WorldInfo](32, 10*time.Minute),
		instances: lru.New[string, c.Instance](4096, 5*time.Minute),
		floors:    lru.New[string, *dungeon.Floor](512, 30*time.Minute),
		spawns:    lru.New[string, []bestiary.Spawn](8192, 30*time.Minute),
	}
}

func (r *Resolver) World(ctx context.Context, id int) (c.WorldInfo, error) {
	if w, ok := r.worlds.Get(id); ok {
		return w, nil
	}
	w, err := bus.Request[c.WorldInfo](ctx, r.bus, c.WorldGet, c.WorldReq{WorldID: id})
	if err != nil {
		return w, err
	}
	r.worlds.Put(id, w)
	return w, nil
}

func (r *Resolver) Instance(ctx context.Context, id string) (c.Instance, error) {
	if in, ok := r.instances.Get(id); ok {
		return in, nil
	}
	in, err := bus.Request[c.Instance](ctx, r.bus, c.DungeonInstance, c.DungeonReq{InstanceID: id})
	if err != nil {
		return in, err
	}
	r.instances.Put(id, in)
	return in, nil
}

// Forget drops a cached instance (after it ends).
func (r *Resolver) Forget(id string) { r.instances.Delete(id) }

func (r *Resolver) Floor(ctx context.Context, z zone.ID) (*dungeon.Floor, c.Instance, error) {
	in, err := r.Instance(ctx, z.Instance)
	if err != nil {
		return nil, in, err
	}
	if z.Floor < 0 || z.Floor >= in.Spec.Floors() {
		return nil, in, apperr.New(apperr.NotFound, "no such floor")
	}
	key := fmt.Sprintf("%s:%d", z.Instance, z.Floor)
	if f, ok := r.floors.Get(key); ok {
		return f, in, nil
	}
	f := dungeon.Generate(in.Spec, z.Floor, z.Instance)
	r.floors.Put(key, f)
	return f, in, nil
}

// WorldSeed of the world a zone belongs to (for species lookups).
func (r *Resolver) WorldSeed(ctx context.Context, z zone.ID) (uint64, error) {
	if z.Kind == zone.Dungeon {
		in, err := r.Instance(ctx, z.Instance)
		return in.Spec.WorldSeed, err
	}
	w, err := r.World(ctx, z.World)
	return w.Seed, err
}

// Tile returns the ground and object of a tile.
func (r *Resolver) Tile(ctx context.Context, z zone.ID, x, y int) (world.Ground, world.Object, error) {
	if z.Kind == zone.Dungeon {
		f, _, err := r.Floor(ctx, z)
		if err != nil {
			return 0, 0, err
		}
		g, o := f.Tile(x, y)
		return g, o, nil
	}
	w, err := r.World(ctx, z.World)
	if err != nil {
		return 0, 0, err
	}
	if !w.InBounds(x, y) {
		return world.GDeepWater, world.ONone, nil
	}
	g, o := w.Tile(x, y)
	return g, o, nil
}

// Walkable is the authoritative collision check for a point (tile units).
func (r *Resolver) Walkable(ctx context.Context, z zone.ID, x, y float64) (bool, error) {
	if x < 0 || y < 0 {
		return false, nil
	}
	g, o, err := r.Tile(ctx, z, int(x), int(y))
	if err != nil {
		return false, err
	}
	return g.Walkable() && !o.Blocking(), nil
}

// PathClear samples the segment between two points for collisions.
func (r *Resolver) PathClear(ctx context.Context, z zone.ID, x0, y0, x1, y1 float64) (bool, error) {
	dx, dy := x1-x0, y1-y0
	steps := int((abs(dx)+abs(dy))/0.4) + 1
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		ok, err := r.Walkable(ctx, z, x0+dx*t, y0+dy*t)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// AreaSpawns lists the monsters of one area.
func (r *Resolver) AreaSpawns(ctx context.Context, z zone.ID, ax, ay int) ([]bestiary.Spawn, error) {
	if z.Kind == zone.Dungeon {
		f, _, err := r.Floor(ctx, z)
		if err != nil {
			return nil, err
		}
		return f.Monsters, nil
	}
	key := fmt.Sprintf("%d:%d:%d", z.World, ax, ay)
	if s, ok := r.spawns.Get(key); ok {
		return s, nil
	}
	w, err := r.World(ctx, z.World)
	if err != nil {
		return nil, err
	}
	s := bestiary.ChunkSpawns(w.Params, ax, ay)
	r.spawns.Put(key, s)
	return s, nil
}

// Monster finds a monster spawn by id inside a zone.
func (r *Resolver) Monster(ctx context.Context, z zone.ID, id string) (bestiary.Spawn, bestiary.Species, error) {
	var list []bestiary.Spawn
	var err error
	if z.Kind == zone.Dungeon {
		if !strings.HasPrefix(id, z.Instance+"_f"+strconv.Itoa(z.Floor)+"_") {
			return bestiary.Spawn{}, bestiary.Species{}, apperr.New(apperr.NotFound, "monster not in this zone")
		}
		list, err = r.AreaSpawns(ctx, z, 0, 0)
	} else {
		// overworld ids: w<world>_<cx>_<cy>_<i>
		parts := strings.Split(strings.TrimPrefix(id, "w"), "_")
		if len(parts) != 4 || parts[0] != strconv.Itoa(z.World) {
			return bestiary.Spawn{}, bestiary.Species{}, apperr.New(apperr.NotFound, "monster not in this zone")
		}
		cx, e1 := strconv.Atoi(parts[1])
		cy, e2 := strconv.Atoi(parts[2])
		if e1 != nil || e2 != nil {
			return bestiary.Spawn{}, bestiary.Species{}, apperr.New(apperr.NotFound, "bad monster id")
		}
		list, err = r.AreaSpawns(ctx, z, cx, cy)
	}
	if err != nil {
		return bestiary.Spawn{}, bestiary.Species{}, err
	}
	for _, s := range list {
		if s.ID == id {
			ws, err := r.WorldSeed(ctx, z)
			if err != nil {
				return s, bestiary.Species{}, err
			}
			sp, ok := bestiary.Get(ws, s.Species)
			if !ok {
				return s, sp, apperr.New(apperr.NotFound, "unknown species")
			}
			return s, sp, nil
		}
	}
	return bestiary.Spawn{}, bestiary.Species{}, apperr.New(apperr.NotFound, "monster not found")
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
