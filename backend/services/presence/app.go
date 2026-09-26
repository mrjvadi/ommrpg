package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/progression"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/zone"
	"github.com/mrjvadi/ommrpg/backend/pkg/hot"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/metrics"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

const (
	posTTL       = 24 * 3600 // seconds
	activeWindow = 90 * 1000 // ms: idle players disappear from "nearby"
	persistEvery = 5 * time.Second
	speedSlack   = 1.15 // bucket refill = speed * slack
	burstSeconds = 1.0  // bucket capacity = speed * burstSeconds + 0.5 tiles
)

type app struct {
	rdb    *redis.Client
	bus    *bus.Bus
	zones  *zones.Resolver
	cf     *centrifugo.Client
	log    *slog.Logger
	speeds *lru.Cache[string, float64]
	stats  *metrics.Counters
}

func newApp(rdb *redis.Client, b *bus.Bus, z *zones.Resolver, cf *centrifugo.Client, m *metrics.Counters, log *slog.Logger) *app {
	return &app{rdb: rdb, bus: b, zones: z, cf: cf, log: log, stats: m, speeds: lru.New[string, float64](8192, 10*time.Second)}
}

const posPrefix = "pos:"

func posKey(id string) string { return posPrefix + id }

func areaKey(z string, ax, ay int) string { return fmt.Sprintf("area:%s:%d:%d", z, ax, ay) }

func areaOf(zs string, x, y float64) string {
	z, _ := zone.Parse(zs)
	ax, ay := z.Area(x, y)
	return areaKey(zs, ax, ay)
}

func toPosition(id string, p hot.Pos) c.Position {
	return c.Position{CharacterID: id, Zone: p.Zone, X: p.X, Y: p.Y, Dir: p.Dir, Anim: p.Anim, At: p.At}
}

func (a *app) load(ctx context.Context, id string) (*hot.Pos, error) {
	p, ok, err := hot.LoadPos(ctx, a.rdb, posKey(id))
	if err != nil || !ok {
		return nil, err
	}
	return &p, nil
}

func (a *app) speed(ctx context.Context, id string) float64 {
	if v, ok := a.speeds.Get(id); ok {
		return v
	}
	prof, err := bus.Request[c.CombatProfile](ctx, a.bus, c.CharacterCombatProfile, c.CharacterReq{CharacterID: id})
	v := progression.BaseMoveSpeed
	if err == nil && prof.Derived.MoveSpeed > 0 {
		v = prof.Derived.MoveSpeed
	}
	a.speeds.Put(id, v)
	return v
}

func bucketCap(speed float64) float64 { return speed*burstSeconds + 0.5 }

// place writes a position unconditionally (enter / teleport / respawn).
func (a *app) place(ctx context.Context, id string, old *hot.Pos, p hot.Pos) error {
	oldArea := ""
	if old != nil {
		oldArea = areaOf(old.Zone, old.X, old.Y)
	}
	_, err := hot.Place(ctx, a.rdb, posKey(id), oldArea, areaOf(p.Zone, p.X, p.Y), id, p, bucketCap(a.speed(ctx, id)), posTTL)
	return err
}

// enter restores the persisted location of a character into the hot store.
func (a *app) enter(ctx context.Context, req c.CharacterReq) (c.Position, error) {
	cur, err := a.load(ctx, req.CharacterID)
	if err != nil {
		return c.Position{}, err
	}
	if cur != nil && a.zoneAlive(ctx, cur.Zone) {
		p := *cur
		return toPosition(req.CharacterID, p), a.place(ctx, req.CharacterID, cur, p)
	}
	loc, err := bus.Request[c.Location](ctx, a.bus, c.CharacterLocationGet, req)
	if err != nil {
		return c.Position{}, err
	}
	p := hot.Pos{Zone: loc.Zone, X: loc.X, Y: loc.Y, Dir: "down", Anim: "idle"}
	if !a.zoneAlive(ctx, p.Zone) {
		// a dungeon instance that no longer exists: back to the world spawn
		prof, err := bus.Request[c.CombatProfile](ctx, a.bus, c.CharacterCombatProfile, req)
		if err != nil {
			return c.Position{}, err
		}
		w, err := a.zones.World(ctx, prof.WorldID)
		if err != nil {
			return c.Position{}, err
		}
		p.Zone, p.X, p.Y = zone.World(w.ID).String(), float64(w.SpawnX)+0.5, float64(w.SpawnY)+1.5
	}
	p.At = time.Now().UnixMilli()
	return toPosition(req.CharacterID, p), a.place(ctx, req.CharacterID, cur, p)
}

func (a *app) zoneAlive(ctx context.Context, zs string) bool {
	z, err := zone.Parse(zs)
	if err != nil {
		return false
	}
	if z.Kind == zone.Dungeon {
		_, err := a.zones.Instance(ctx, z.Instance)
		return err == nil
	}
	return true
}

func (a *app) get(ctx context.Context, req c.CharacterReq) (c.Position, error) {
	p, err := a.load(ctx, req.CharacterID)
	if err != nil {
		return c.Position{}, err
	}
	if p == nil {
		return c.Position{}, apperr.New(apperr.NotFound, "character is not in the world")
	}
	return toPosition(req.CharacterID, *p), nil
}

var validDirs = map[string]bool{"up": true, "down": true, "left": true, "right": true}
var validAnims = map[string]bool{"idle": true, "walk": true, "slash": true, "thrust": true, "shoot": true, "spellcast": true, "hurt": true}

// move validates collisions here (it needs the seed-generated map) and the
// speed/ordering/concurrency atomically in Dragonfly (move.lua). A version
// conflict (someone teleported us meanwhile) is retried once.
func (a *app) move(ctx context.Context, req c.MoveReq) (c.MoveResp, error) {
	if math.IsNaN(req.X) || math.IsNaN(req.Y) || math.IsInf(req.X, 0) || math.IsInf(req.Y, 0) {
		return c.MoveResp{}, apperr.New(apperr.Invalid, "bad coordinates")
	}
	for attempt := 0; attempt < 2; attempt++ {
		cur, err := a.load(ctx, req.CharacterID)
		if err != nil {
			return c.MoveResp{}, err
		}
		if cur == nil {
			return c.MoveResp{}, apperr.New(apperr.Conflict, "enter the world first")
		}
		dir, anim := req.Dir, req.Anim
		if !validDirs[dir] {
			dir = cur.Dir
		}
		if !validAnims[anim] {
			anim = "walk"
		}
		z, err := zone.Parse(cur.Zone)
		if err != nil {
			return c.MoveResp{}, err
		}
		clear, err := a.zones.PathClear(ctx, z, cur.X, cur.Y, req.X, req.Y)
		if err != nil {
			return c.MoveResp{}, err
		}
		if !clear {
			return c.MoveResp{Accepted: false, Position: toPosition(req.CharacterID, *cur)}, nil
		}
		sp := a.speed(ctx, req.CharacterID)
		st, _, x, y, err := hot.Move(ctx, a.rdb, hot.MoveArgs{
			PosKey: posKey(req.CharacterID), OldArea: areaOf(cur.Zone, cur.X, cur.Y), NewArea: areaOf(cur.Zone, req.X, req.Y),
			Member: req.CharacterID, ExpectedVer: cur.Ver, X: req.X, Y: req.Y, Dir: dir, Anim: anim,
			Speed: sp * speedSlack, Cap: bucketCap(sp), Seq: req.Seq, TTLSeconds: posTTL,
		})
		if err != nil {
			return c.MoveResp{}, err
		}
		switch st {
		case hot.MoveConflict:
			continue
		case hot.MoveStale:
			return c.MoveResp{Accepted: true, Position: toPosition(req.CharacterID, *cur)}, nil
		case hot.MoveTooFast:
			a.stats.Inc("move_rejected", 1)
			return c.MoveResp{Accepted: false, Position: toPosition(req.CharacterID, *cur)}, nil
		}
		a.stats.Inc("moves", 1)
		next := hot.Pos{Zone: cur.Zone, X: x, Y: y, Dir: dir, Anim: anim, At: time.Now().UnixMilli()}
		a.broadcast(ctx, z, cur, next, req.CharacterID, map[string]any{"t": "mv", "id": req.CharacterID, "x": round2(x), "y": round2(y), "d": dir, "a": anim, "s": req.Seq})
		if z.Kind == zone.Overworld {
			a.persist(req.CharacterID, next)
		}
		return c.MoveResp{Accepted: true, Position: toPosition(req.CharacterID, next)}, nil
	}
	return c.MoveResp{}, apperr.New(apperr.Conflict, "position changed, retry")
}

func (a *app) persist(id string, p hot.Pos) {
	if set, _ := a.rdb.SetNX(context.Background(), "persist:"+id, 1, persistEvery).Result(); !set {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := bus.Request[struct{}](ctx, a.bus, c.CharacterLocationSet, c.Location{CharacterID: id, Zone: p.Zone, X: p.X, Y: p.Y}); err != nil {
			a.log.Warn("persist location", "err", err)
		}
	}()
}

// broadcast publishes to the new area and, when the mover crossed an area
// border, to the old one too so watchers there see them leave.
func (a *app) broadcast(ctx context.Context, z zone.ID, old *hot.Pos, next hot.Pos, _ string, msg map[string]any) {
	ax, ay := z.Area(next.X, next.Y)
	chans := []string{z.Channel(ax, ay)}
	if old != nil && old.Zone == next.Zone {
		ox, oy := z.Area(old.X, old.Y)
		if ox != ax || oy != ay {
			chans = append(chans, z.Channel(ox, oy))
		}
	}
	if err := a.cf.Broadcast(ctx, chans, msg); err != nil {
		a.log.Warn("broadcast", "err", err)
	}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func (a *app) nearby(ctx context.Context, req c.CharacterReq) (c.NearbyResp, error) {
	cur, err := a.load(ctx, req.CharacterID)
	if err != nil || cur == nil {
		return c.NearbyResp{Players: []c.Position{}}, err
	}
	z, _ := zone.Parse(cur.Zone)
	ax, ay := z.Area(cur.X, cur.Y)
	var keys []string
	for _, ar := range z.NeighbourAreas(ax, ay) {
		keys = append(keys, areaKey(cur.Zone, ar[0], ar[1]))
	}
	ps, ids, err := hot.Nearby(ctx, a.rdb, keys, activeWindow, req.CharacterID, cur.Zone, posPrefix)
	if err != nil {
		return c.NearbyResp{}, err
	}
	out := c.NearbyResp{Players: make([]c.Position, 0, len(ps))}
	for i, p := range ps {
		out.Players = append(out.Players, toPosition(ids[i], p))
	}
	return out, nil
}

func (a *app) teleport(ctx context.Context, req c.TeleportReq) (c.Position, error) {
	z, err := zone.Parse(req.Zone)
	if err != nil {
		return c.Position{}, apperr.New(apperr.Invalid, "%v", err)
	}
	old, err := a.load(ctx, req.CharacterID)
	if err != nil {
		return c.Position{}, err
	}
	next := hot.Pos{Zone: req.Zone, X: req.X, Y: req.Y, Dir: "down", Anim: "idle", At: time.Now().UnixMilli()}
	if err := a.place(ctx, req.CharacterID, old, next); err != nil {
		return c.Position{}, err
	}
	if old != nil {
		oz, _ := zone.Parse(old.Zone)
		ox, oy := oz.Area(old.X, old.Y)
		_ = a.cf.Publish(ctx, oz.Channel(ox, oy), map[string]any{"t": "gone", "id": req.CharacterID})
	}
	ax, ay := z.Area(next.X, next.Y)
	_ = a.cf.Publish(ctx, z.Channel(ax, ay), map[string]any{"t": "mv", "id": req.CharacterID, "x": next.X, "y": next.Y, "d": next.Dir, "a": next.Anim})
	_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(req.CharacterID), map[string]any{"t": "teleport", "zone": next.Zone, "x": next.X, "y": next.Y, "reason": req.Reason})
	if z.Kind == zone.Overworld {
		_, _ = bus.Request[struct{}](ctx, a.bus, c.CharacterLocationSet, c.Location{CharacterID: req.CharacterID, Zone: next.Zone, X: next.X, Y: next.Y})
	}
	return toPosition(req.CharacterID, next), nil
}

func (a *app) look(ctx context.Context, req c.CharacterReq) (struct{}, error) {
	a.speeds.Delete(req.CharacterID)
	p, err := a.load(ctx, req.CharacterID)
	if err != nil || p == nil {
		return struct{}{}, err
	}
	z, _ := zone.Parse(p.Zone)
	ax, ay := z.Area(p.X, p.Y)
	return struct{}{}, a.cf.Publish(ctx, z.Channel(ax, ay), map[string]any{"t": "look", "id": req.CharacterID})
}

// stats reports live population for the admin panel: players active in the
// last `activeWindow`, per zone.
func (a *app) statsHandler(ctx context.Context, _ struct{}) (c.PresenceStats, error) {
	out := c.PresenceStats{Zones: map[string]int{}}
	var cursor uint64
	seen := map[string]bool{}
	min := fmt.Sprint(time.Now().UnixMilli() - activeWindow)
	for {
		keys, next, err := a.rdb.Scan(ctx, cursor, "area:*", 500).Result()
		if err != nil {
			return out, err
		}
		for _, k := range keys {
			ids, _ := a.rdb.ZRangeByScore(ctx, k, &redis.ZRangeBy{Min: min, Max: "+inf"}).Result()
			// key format area:<zone>:<ax>:<ay>, zone itself contains ':'
			zs := k[len("area:"):]
			for i := 0; i < 2; i++ {
				if j := lastColon(zs); j >= 0 {
					zs = zs[:j]
				}
			}
			for _, id := range ids {
				if !seen[id] {
					seen[id] = true
					out.Zones[zs]++
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	out.Online = len(seen)
	return out, nil
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
