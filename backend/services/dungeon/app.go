package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/dungeon"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/items"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/zone"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

const (
	instanceTTL = 2 * time.Hour
	reach       = 1.9 // tiles a character may be from something to use it
)

type app struct {
	rdb    *redis.Client
	bus    *bus.Bus
	zones  *zones.Resolver
	cf     *centrifugo.Client
	log    *slog.Logger
	secret uint64
}

func instKey(id string) string     { return "dng:" + id }
func activeKey(char string) string { return "dng:active:" + char }

func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (a *app) loadInstance(ctx context.Context, id string) (c.Instance, error) {
	raw, err := a.rdb.Get(ctx, instKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return c.Instance{}, apperr.New(apperr.NotFound, "dungeon instance ended")
	}
	if err != nil {
		return c.Instance{}, err
	}
	var in c.Instance
	return in, json.Unmarshal(raw, &in)
}

func (a *app) instance(ctx context.Context, req c.DungeonReq) (c.Instance, error) {
	return a.loadInstance(ctx, req.InstanceID)
}

func near(p c.Position, x, y int) bool {
	return math.Hypot(p.X-(float64(x)+0.5), p.Y-(float64(y)+0.5)) <= reach
}

func (a *app) position(ctx context.Context, char string) (c.Position, zone.ID, error) {
	p, err := bus.Request[c.Position](ctx, a.bus, c.PresenceGet, c.CharacterReq{CharacterID: char})
	if err != nil {
		return p, zone.ID{}, err
	}
	z, err := zone.Parse(p.Zone)
	return p, z, err
}

func (a *app) enter(ctx context.Context, req c.DungeonEnterReq) (c.DungeonView, error) {
	pos, z, err := a.position(ctx, req.CharacterID)
	if err != nil {
		return c.DungeonView{}, err
	}
	if z.Kind != zone.Overworld {
		return c.DungeonView{}, apperr.New(apperr.Conflict, "you are already inside a dungeon")
	}
	parts := strings.Split(req.EntranceID, "_")
	if len(parts) != 3 || parts[0] != strconv.Itoa(z.World) {
		return c.DungeonView{}, apperr.New(apperr.Invalid, "unknown entrance")
	}
	cx, _ := strconv.Atoi(parts[1])
	cy, _ := strconv.Atoi(parts[2])
	w, err := a.zones.World(ctx, z.World)
	if err != nil {
		return c.DungeonView{}, err
	}
	ent, ok := w.EntranceIn(cx, cy)
	if !ok || ent.ID != req.EntranceID {
		return c.DungeonView{}, apperr.New(apperr.NotFound, "no entrance here")
	}
	if !near(pos, ent.X, ent.Y) {
		return c.DungeonView{}, apperr.New(apperr.Invalid, "walk up to the entrance first")
	}
	in := c.Instance{
		ID:         newID(),
		Spec:       dungeon.Spec{Seed: seed.Derive(w.Seed, "dungeon", ent.ID), WorldSeed: w.Seed, Tier: ent.Tier, Name: ent.Name},
		EntranceID: ent.ID, WorldID: w.ID, OwnerID: req.CharacterID,
		// return point: the tile below the entrance (never the client's choice)
		ReturnX: float64(ent.X) + 0.5, ReturnY: float64(ent.Y) + 1.5,
		CreatedAt: time.Now().UTC(),
	}
	if !w.Walkable(ent.X, ent.Y+1) {
		in.ReturnX, in.ReturnY = pos.X, pos.Y
	}
	raw, _ := json.Marshal(in)
	if err := a.rdb.Set(ctx, instKey(in.ID), raw, instanceTTL).Err(); err != nil {
		return c.DungeonView{}, err
	}
	if err := a.rdb.Set(ctx, activeKey(req.CharacterID), in.ID, instanceTTL).Err(); err != nil {
		return c.DungeonView{}, err
	}
	_ = a.bus.Publish(ctx, c.EvDungeonEntered, "dungeon-entered:"+in.ID, c.DungeonEnteredEv{
		CharacterID: req.CharacterID, InstanceID: in.ID, EntranceID: ent.ID, Name: ent.Name, Tier: ent.Tier})
	return a.moveTo(ctx, req.CharacterID, in, 0, "dungeon_enter")
}

func (a *app) moveTo(ctx context.Context, char string, in c.Instance, floor int, reason string) (c.DungeonView, error) {
	z := zone.DungeonFloor(in.ID, floor)
	f, _, err := a.zones.Floor(ctx, z)
	if err != nil {
		return c.DungeonView{}, err
	}
	p, err := bus.Request[c.Position](ctx, a.bus, c.PresenceTeleport, c.TeleportReq{
		CharacterID: char, Zone: z.String(), X: float64(f.Start.X) + 0.5, Y: float64(f.Start.Y) + 0.5, Reason: reason})
	if err != nil {
		return c.DungeonView{}, err
	}
	return c.DungeonView{InstanceID: in.ID, Name: in.Spec.Name, Tier: in.Spec.Tier, Zone: z.String(), Floor: f, Position: p}, nil
}

// current returns the instance/floor the character stands in.
func (a *app) current(ctx context.Context, char string) (c.Position, zone.ID, c.Instance, *dungeon.Floor, error) {
	pos, z, err := a.position(ctx, char)
	if err != nil {
		return pos, z, c.Instance{}, nil, err
	}
	if z.Kind != zone.Dungeon {
		return pos, z, c.Instance{}, nil, apperr.New(apperr.Conflict, "you are not in a dungeon")
	}
	in, err := a.loadInstance(ctx, z.Instance)
	if err != nil {
		return pos, z, in, nil, err
	}
	if in.OwnerID != char {
		return pos, z, in, nil, apperr.New(apperr.Forbidden, "not your dungeon")
	}
	f, _, err := a.zones.Floor(ctx, z)
	return pos, z, in, f, err
}

func (a *app) floor(ctx context.Context, req c.DungeonReq) (c.DungeonView, error) {
	pos, z, in, f, err := a.current(ctx, req.CharacterID)
	if err != nil {
		return c.DungeonView{}, err
	}
	return c.DungeonView{InstanceID: in.ID, Name: in.Spec.Name, Tier: in.Spec.Tier, Zone: z.String(), Floor: f, Position: pos}, nil
}

func (a *app) descend(ctx context.Context, req c.DungeonReq) (c.DungeonView, error) {
	pos, z, in, f, err := a.current(ctx, req.CharacterID)
	if err != nil {
		return c.DungeonView{}, err
	}
	if f.Stairs == nil || !near(pos, f.Stairs.X, f.Stairs.Y) {
		return c.DungeonView{}, apperr.New(apperr.Invalid, "there are no stairs here")
	}
	return a.moveTo(ctx, req.CharacterID, in, z.Floor+1, "dungeon_descend")
}

// leave returns the character to the entrance it came from. InstanceID
// "force" (used internally on death) skips the "stand on the exit" check.
func (a *app) leave(ctx context.Context, req c.DungeonReq) (c.LeaveResp, error) {
	pos, _, in, f, err := a.current(ctx, req.CharacterID)
	if err != nil {
		return c.LeaveResp{}, err
	}
	if req.InstanceID != "force" && !near(pos, f.Exit.X, f.Exit.Y) {
		return c.LeaveResp{}, apperr.New(apperr.Invalid, "find the exit to leave")
	}
	p, err := bus.Request[c.Position](ctx, a.bus, c.PresenceTeleport, c.TeleportReq{
		CharacterID: req.CharacterID, Zone: zone.World(in.WorldID).String(), X: in.ReturnX, Y: in.ReturnY, Reason: "dungeon_leave"})
	if err != nil {
		return c.LeaveResp{}, err
	}
	a.rdb.Del(ctx, activeKey(req.CharacterID))
	return c.LeaveResp{Position: p}, nil
}

func (a *app) openChest(ctx context.Context, req c.DungeonReq) (c.ChestResp, error) {
	pos, z, in, f, err := a.current(ctx, req.CharacterID)
	if err != nil {
		return c.ChestResp{}, err
	}
	var chest *dungeon.Chest
	for i := range f.Chests {
		if f.Chests[i].ID == req.ChestID {
			chest = &f.Chests[i]
		}
	}
	if chest == nil {
		return c.ChestResp{}, apperr.New(apperr.NotFound, "no such chest")
	}
	if !near(pos, chest.X, chest.Y) {
		return c.ChestResp{}, apperr.New(apperr.Invalid, "walk up to the chest first")
	}
	ok, err := a.rdb.SetNX(ctx, "chest:"+chest.ID, req.CharacterID, instanceTTL).Result()
	if err != nil {
		return c.ChestResp{}, err
	}
	if !ok {
		return c.ChestResp{}, apperr.New(apperr.Conflict, "the chest is empty")
	}
	loot := items.RollLoot(seed.Derive(a.secret, "chest", chest.ID), in.Spec.Level(z.Floor), "chest", 0)
	if err := a.bus.Publish(ctx, c.EvChestOpened, "chest:"+chest.ID, c.ChestOpenedEv{CharacterID: req.CharacterID, ChestID: chest.ID, Loot: loot}); err != nil {
		return c.ChestResp{}, err
	}
	ch := z.Channel(0, 0)
	_ = a.cf.Publish(ctx, ch, map[string]any{"t": "chest", "id": chest.ID, "x": chest.X, "y": chest.Y})
	return c.ChestResp{Loot: loot}, nil
}

// onKill detects boss kills and marks the dungeon as cleared.
func (a *app) onKill(ctx context.Context, ev bus.Event) error {
	d, err := bus.Decode[c.MonsterKilledEv](ev)
	if err != nil || !strings.HasSuffix(d.MonsterID, "_boss") {
		return nil
	}
	z, err := zone.Parse(d.Zone)
	if err != nil || z.Kind != zone.Dungeon {
		return nil
	}
	in, err := a.loadInstance(ctx, z.Instance)
	if apperr.Is(err, apperr.NotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	f, _, err := a.zones.Floor(ctx, z)
	if err != nil || f.BossID != d.MonsterID {
		return nil
	}
	if err := a.bus.Publish(ctx, c.EvDungeonCleared, "cleared:"+in.ID, c.DungeonClearedEv{
		CharacterID: d.CharacterID, InstanceID: in.ID, EntranceID: in.EntranceID, Name: in.Spec.Name, Tier: in.Spec.Tier}); err != nil {
		return err
	}
	_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(d.CharacterID), map[string]any{"t": "dungeon_cleared", "name": in.Spec.Name, "tier": in.Spec.Tier})
	return nil
}
