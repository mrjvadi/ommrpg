package main

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

const (
	posTTL        = 24 * time.Hour
	activeWindow  = 90 * time.Second // players idle longer disappear from "nearby"
	persistEvery  = 5 * time.Second
	speedSlack    = 1.3
	distanceSlack = 0.45
)

type app struct {
	rdb    *redis.Client
	bus    *bus.Bus
	zones  *zones.Resolver
	cf     *centrifugo.Client
	log    *slog.Logger
	speeds *lru.Cache[string, float64]
}

func newApp(rdb *redis.Client, b *bus.Bus, z *zones.Resolver, cf *centrifugo.Client, log *slog.Logger) *app {
	return &app{rdb: rdb, bus: b, zones: z, cf: cf, log: log, speeds: lru.New[string, float64](8192, 10*time.Second)}
}

func posKey(id string) string { return "pos:" + id }

func areaKey(z string, ax, ay int) string { return fmt.Sprintf("area:%s:%d:%d", z, ax, ay) }

func (a *app) load(ctx context.Context, id string) (*c.Position, error) {
	raw, err := a.rdb.Get(ctx, posKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p c.Position
	return &p, json.Unmarshal(raw, &p)
}

func (a *app) save(ctx context.Context, p c.Position, old *c.Position) error {
	raw, _ := json.Marshal(p)
	z, _ := zone.Parse(p.Zone)
	ax, ay := z.Area(p.X, p.Y)
	pipe := a.rdb.TxPipeline()
	pipe.Set(ctx, posKey(p.CharacterID), raw, posTTL)
	if old != nil {
		oz, _ := zone.Parse(old.Zone)
		ox, oy := oz.Area(old.X, old.Y)
		if old.Zone != p.Zone || ox != ax || oy != ay {
			pipe.ZRem(ctx, areaKey(old.Zone, ox, oy), p.CharacterID)
		}
	}
	pipe.ZAdd(ctx, areaKey(p.Zone, ax, ay), redis.Z{Score: float64(p.At), Member: p.CharacterID})
	pipe.Expire(ctx, areaKey(p.Zone, ax, ay), posTTL)
	_, err := pipe.Exec(ctx)
	return err
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

// enter restores the persisted location of a character into the hot store.
func (a *app) enter(ctx context.Context, req c.CharacterReq) (c.Position, error) {
	if p, err := a.load(ctx, req.CharacterID); err != nil {
		return c.Position{}, err
	} else if p != nil {
		if a.zoneAlive(ctx, p.Zone) {
			p.At = time.Now().UnixMilli()
			return *p, a.save(ctx, *p, p)
		}
	}
	loc, err := bus.Request[c.Location](ctx, a.bus, c.CharacterLocationGet, req)
	if err != nil {
		return c.Position{}, err
	}
	p := c.Position{CharacterID: req.CharacterID, Zone: loc.Zone, X: loc.X, Y: loc.Y, Dir: "down", Anim: "idle", At: time.Now().UnixMilli()}
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
	return p, a.save(ctx, p, nil)
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
	return *p, nil
}

var validDirs = map[string]bool{"up": true, "down": true, "left": true, "right": true}
var validAnims = map[string]bool{"idle": true, "walk": true, "slash": true, "thrust": true, "shoot": true, "spellcast": true, "hurt": true}

func (a *app) move(ctx context.Context, req c.MoveReq) (c.MoveResp, error) {
	cur, err := a.load(ctx, req.CharacterID)
	if err != nil {
		return c.MoveResp{}, err
	}
	if cur == nil {
		return c.MoveResp{}, apperr.New(apperr.Conflict, "enter the world first")
	}
	if math.IsNaN(req.X) || math.IsNaN(req.Y) || math.IsInf(req.X, 0) || math.IsInf(req.Y, 0) {
		return c.MoveResp{Position: *cur}, nil
	}
	if !validDirs[req.Dir] {
		req.Dir = cur.Dir
	}
	if !validAnims[req.Anim] {
		req.Anim = "walk"
	}
	z, err := zone.Parse(cur.Zone)
	if err != nil {
		return c.MoveResp{}, err
	}
	now := time.Now().UnixMilli()
	dt := math.Min(2, math.Max(0.05, float64(now-cur.At)/1000))
	maxDist := a.speed(ctx, req.CharacterID)*dt*speedSlack + distanceSlack
	dist := math.Hypot(req.X-cur.X, req.Y-cur.Y)
	ok := dist <= maxDist
	if ok && dist > 0 {
		ok, err = a.zones.PathClear(ctx, z, cur.X, cur.Y, req.X, req.Y)
		if err != nil {
			return c.MoveResp{}, err
		}
	}
	if !ok {
		// reject: the client must snap back to the authoritative position
		cur.At = now
		_ = a.save(ctx, *cur, cur)
		return c.MoveResp{Accepted: false, Position: *cur}, nil
	}
	next := c.Position{CharacterID: req.CharacterID, Zone: cur.Zone, X: req.X, Y: req.Y, Dir: req.Dir, Anim: req.Anim, At: now}
	if err := a.save(ctx, next, cur); err != nil {
		return c.MoveResp{}, err
	}
	a.broadcast(ctx, z, cur, next, map[string]any{"t": "mv", "id": next.CharacterID, "x": round2(next.X), "y": round2(next.Y), "d": next.Dir, "a": next.Anim})
	if z.Kind == zone.Overworld {
		if set, _ := a.rdb.SetNX(ctx, "persist:"+req.CharacterID, 1, persistEvery).Result(); set {
			go func() {
				pctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_, err := bus.Request[struct{}](pctx, a.bus, c.CharacterLocationSet, c.Location{CharacterID: next.CharacterID, Zone: next.Zone, X: next.X, Y: next.Y})
				if err != nil {
					a.log.Warn("persist location", "err", err)
				}
			}()
		}
	}
	return c.MoveResp{Accepted: true, Position: next}, nil
}

// broadcast publishes to the new area and, when the mover crossed an area
// border, to the old one too so watchers there see them leave.
func (a *app) broadcast(ctx context.Context, z zone.ID, old *c.Position, next c.Position, msg map[string]any) {
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
	minScore := fmt.Sprintf("%d", time.Now().Add(-activeWindow).UnixMilli())
	var ids []string
	for _, ar := range z.NeighbourAreas(ax, ay) {
		key := areaKey(cur.Zone, ar[0], ar[1])
		a.rdb.ZRemRangeByScore(ctx, key, "-inf", "("+minScore)
		m, err := a.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{Min: minScore, Max: "+inf"}).Result()
		if err != nil {
			return c.NearbyResp{}, err
		}
		ids = append(ids, m...)
	}
	out := c.NearbyResp{Players: []c.Position{}}
	for _, id := range ids {
		if id == req.CharacterID {
			continue
		}
		p, err := a.load(ctx, id)
		if err == nil && p != nil && p.Zone == cur.Zone {
			out.Players = append(out.Players, *p)
		}
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
	next := c.Position{CharacterID: req.CharacterID, Zone: req.Zone, X: req.X, Y: req.Y, Dir: "down", Anim: "idle", At: time.Now().UnixMilli()}
	if err := a.save(ctx, next, old); err != nil {
		return c.Position{}, err
	}
	if old != nil {
		oz, _ := zone.Parse(old.Zone)
		ox, oy := oz.Area(old.X, old.Y)
		_ = a.cf.Publish(ctx, oz.Channel(ox, oy), map[string]any{"t": "gone", "id": req.CharacterID})
	}
	ax, ay := z.Area(next.X, next.Y)
	_ = a.cf.Publish(ctx, z.Channel(ax, ay), map[string]any{"t": "mv", "id": next.CharacterID, "x": next.X, "y": next.Y, "d": next.Dir, "a": next.Anim})
	_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(req.CharacterID), map[string]any{"t": "teleport", "zone": next.Zone, "x": next.X, "y": next.Y, "reason": req.Reason})
	if z.Kind == zone.Overworld {
		_, _ = bus.Request[struct{}](ctx, a.bus, c.CharacterLocationSet, c.Location{CharacterID: next.CharacterID, Zone: next.Zone, X: next.X, Y: next.Y})
	}
	return next, nil
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
