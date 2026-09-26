package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/bestiary"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/combat"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/items"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/zone"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

type app struct {
	rdb      *redis.Client
	bus      *bus.Bus
	zones    *zones.Resolver
	cf       *centrifugo.Client
	secret   uint64
	log      *slog.Logger
	profiles *lru.Cache[string, c.CombatProfile]
}

func newApp(rdb *redis.Client, b *bus.Bus, z *zones.Resolver, cf *centrifugo.Client, secret uint64, log *slog.Logger) *app {
	return &app{rdb: rdb, bus: b, zones: z, cf: cf, secret: secret, log: log, profiles: lru.New[string, c.CombatProfile](8192, 3*time.Second)}
}

func (a *app) profile(ctx context.Context, id string) (c.CombatProfile, error) {
	if p, ok := a.profiles.Get(id); ok {
		return p, nil
	}
	p, err := bus.Request[c.CombatProfile](ctx, a.bus, c.CharacterCombatProfile, c.CharacterReq{CharacterID: id})
	if err == nil {
		a.profiles.Put(id, p)
	}
	return p, err
}

func monKey(z, id string) string { return "mon:" + z + ":" + id }

// damageScript applies damage to a monster exactly once. It returns
// {state, hp, deadUntil}: state 0 = hit, 1 = killed by this hit, 2 = already dead.
var damageScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local maxhp = tonumber(ARGV[2])
local dmg = tonumber(ARGV[3])
local respawn = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])
local dead = tonumber(redis.call('HGET', key, 'dead_until') or '0')
if dead == -1 or dead > now then
  return {2, 0, dead}
end
local hp = tonumber(redis.call('HGET', key, 'hp') or '-1')
if hp < 0 or dead ~= 0 then hp = maxhp end
hp = hp - dmg
if hp <= 0 then
  local untilv = -1
  if respawn > 0 then untilv = now + respawn end
  redis.call('HSET', key, 'hp', maxhp, 'dead_until', untilv)
  redis.call('EXPIRE', key, ttl)
  return {1, 0, untilv}
end
redis.call('HSET', key, 'hp', hp, 'dead_until', 0)
redis.call('EXPIRE', key, ttl)
return {0, hp, 0}
`)

type vitals struct {
	HP int   `json:"hp"`
	At int64 `json:"at"`
}

func hpKey(id string) string { return "hp:" + id }

// playerHP returns current hp including passive regeneration (2%/s).
func (a *app) playerHP(ctx context.Context, id string, max int) (int, error) {
	raw, err := a.rdb.Get(ctx, hpKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return max, nil
	}
	if err != nil {
		return 0, err
	}
	var v vitals
	_ = json.Unmarshal(raw, &v)
	regen := float64(time.Now().UnixMilli()-v.At) / 1000 * float64(max) * 0.02
	return int(math.Min(float64(max), float64(v.HP)+regen)), nil
}

func (a *app) setHP(ctx context.Context, id string, hp int) error {
	raw, _ := json.Marshal(vitals{HP: hp, At: time.Now().UnixMilli()})
	return a.rdb.Set(ctx, hpKey(id), raw, 24*time.Hour).Err()
}

func (a *app) attack(ctx context.Context, req c.AttackReq) (c.AttackResp, error) {
	prof, err := a.profile(ctx, req.CharacterID)
	if err != nil {
		return c.AttackResp{}, err
	}
	pos, err := bus.Request[c.Position](ctx, a.bus, c.PresenceGet, c.CharacterReq{CharacterID: req.CharacterID})
	if err != nil {
		return c.AttackResp{}, err
	}
	z, err := zone.Parse(pos.Zone)
	if err != nil {
		return c.AttackResp{}, err
	}
	spawn, species, err := a.zones.Monster(ctx, z, req.TargetID)
	if err != nil {
		return c.AttackResp{}, err
	}
	d := prof.Derived
	mx, my := float64(spawn.X)+0.5, float64(spawn.Y)+0.5
	if !combat.InRange(pos.X, pos.Y, mx, my, d.Range) {
		return c.AttackResp{}, apperr.New(apperr.Invalid, "target out of range")
	}
	cdMs := int64(d.Cooldown * 1000 * 0.9) // small tolerance for latency jitter
	if ok, _ := a.rdb.SetNX(ctx, "cd:"+req.CharacterID, 1, time.Duration(cdMs)*time.Millisecond).Result(); !ok {
		return c.AttackResp{}, apperr.New(apperr.Cooldown, "attack is on cooldown")
	}
	st := bestiary.StatsFor(species, spawn.Level, spawn.Rank)
	n, _ := a.rdb.Incr(ctx, "atkseq:"+req.CharacterID).Result()
	hit := combat.Resolve(seed.Derive(a.secret, "attack", req.CharacterID, n), d.Attack, st.Defense, d.CritChance)
	now := time.Now().UnixMilli()
	respawnMs := int64(st.Respawn) * 1000
	res, err := damageScript.Run(ctx, a.rdb, []string{monKey(pos.Zone, spawn.ID)}, now, st.MaxHP, hit.Damage, respawnMs, 6*3600).Int64Slice()
	if err != nil {
		return c.AttackResp{}, err
	}
	if res[0] == 2 {
		return c.AttackResp{}, apperr.New(apperr.Conflict, "target is already dead")
	}
	maxHP := d.MaxHP
	php, err := a.playerHP(ctx, req.CharacterID, maxHP)
	if err != nil {
		return c.AttackResp{}, err
	}
	if d.LifeSteal > 0 {
		php = int(math.Min(float64(maxHP), float64(php)+float64(hit.Damage)*d.LifeSteal))
	}
	out := c.AttackResp{Hit: hit, TargetHP: int(res[1]), TargetMax: st.MaxHP, PlayerMax: maxHP}
	ax, ay := z.Area(mx, my)
	channel := z.Channel(ax, ay)
	if res[0] == 1 {
		out.Killed = true
		xp := float64(st.XP) * prof.Hidden.XPMultiplier() * (1 + d.XPBonus)
		if gap := prof.Level - spawn.Level; gap > 5 {
			xp *= math.Max(0.1, 1-0.15*float64(gap-5)) // farming weak monsters pays little
		}
		out.XP = int64(math.Max(1, math.Round(xp)))
		loot := items.RollLoot(seed.Derive(a.secret, "loot", pos.Zone, spawn.ID, res[2]), spawn.Level, string(spawn.Rank), prof.Hidden.Luck()+d.MagicFind)
		out.Loot = &loot
		ev := c.MonsterKilledEv{
			CharacterID: req.CharacterID, MonsterID: spawn.ID, Species: species.ID, SpeciesName: species.Name,
			Rank: spawn.Rank, Level: spawn.Level, Zone: pos.Zone, XP: out.XP, WeaponKind: d.WeaponKind, Loot: loot,
		}
		if err := a.bus.Publish(ctx, c.EvMonsterKilled, fmt.Sprintf("kill:%s:%s:%d", pos.Zone, spawn.ID, res[2]), ev); err != nil {
			a.log.Error("publish kill", "err", err)
		}
		_ = a.cf.Publish(ctx, channel, map[string]any{"t": "die", "id": spawn.ID, "until": res[2], "by": req.CharacterID, "dmg": hit.Damage, "crit": hit.Crit})
	} else {
		_ = a.cf.Publish(ctx, channel, map[string]any{"t": "hit", "id": spawn.ID, "hp": res[1], "max": st.MaxHP, "by": req.CharacterID, "dmg": hit.Damage, "crit": hit.Crit})
		// counterattack: the monster strikes back when the attacker is in its reach
		if combat.InRange(pos.X, pos.Y, mx, my, st.Range+0.5) {
			cd := time.Duration(st.Cooldown*1000) * time.Millisecond
			if ok, _ := a.rdb.SetNX(ctx, "mcd:"+pos.Zone+":"+spawn.ID+":"+req.CharacterID, 1, cd).Result(); ok {
				m, _ := a.rdb.Incr(ctx, "defseq:"+req.CharacterID).Result()
				counter := combat.Resolve(seed.Derive(a.secret, "counter", req.CharacterID, m), st.Attack, d.Defense, 0.05)
				out.Counter = &counter
				php -= counter.Damage
			}
		}
	}
	if php <= 0 {
		out.Died = true
		php = maxHP
		a.onDeath(ctx, req.CharacterID, prof, z, species.Name)
	}
	if err := a.setHP(ctx, req.CharacterID, php); err != nil {
		return out, err
	}
	out.PlayerHP = php
	return out, nil
}

// onDeath sends the character back to safety: out of a dungeon to its
// entrance (return-bound) or to the world shrine.
func (a *app) onDeath(ctx context.Context, id string, prof c.CombatProfile, z zone.ID, killer string) {
	_ = a.bus.Publish(ctx, c.EvCharacterDied, fmt.Sprintf("died:%s:%d", id, time.Now().UnixNano()), c.CharacterDiedEv{CharacterID: id, Zone: z.String(), KilledBy: killer})
	_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(id), map[string]any{"t": "died", "by": killer})
	if z.Kind == zone.Dungeon {
		if _, err := bus.Request[c.LeaveResp](ctx, a.bus, c.DungeonLeave, c.DungeonReq{CharacterID: id, InstanceID: "force"}); err == nil {
			return
		}
	}
	w, err := a.zones.World(ctx, prof.WorldID)
	if err != nil {
		a.log.Error("respawn: world", "err", err)
		return
	}
	_, err = bus.Request[c.Position](ctx, a.bus, c.PresenceTeleport, c.TeleportReq{
		CharacterID: id, Zone: zone.World(w.ID).String(), X: float64(w.SpawnX) + 0.5, Y: float64(w.SpawnY) + 1.5, Reason: "death"})
	if err != nil {
		a.log.Error("respawn: teleport", "err", err)
	}
}

func (a *app) monsters(ctx context.Context, req c.MonstersReq) (c.MonstersResp, error) {
	pos, err := bus.Request[c.Position](ctx, a.bus, c.PresenceGet, c.CharacterReq{CharacterID: req.CharacterID})
	if err != nil {
		return c.MonstersResp{}, err
	}
	z, err := zone.Parse(pos.Zone)
	if err != nil {
		return c.MonstersResp{}, err
	}
	ws, err := a.zones.WorldSeed(ctx, z)
	if err != nil {
		return c.MonstersResp{}, err
	}
	ax, ay := z.Area(pos.X, pos.Y)
	out := c.MonstersResp{Zone: pos.Zone, Monsters: []c.MonsterState{}}
	var spawns []bestiary.Spawn
	for _, ar := range z.NeighbourAreas(ax, ay) {
		s, err := a.zones.AreaSpawns(ctx, z, ar[0], ar[1])
		if err != nil {
			continue // out of world bounds
		}
		spawns = append(spawns, s...)
	}
	pipe := a.rdb.Pipeline()
	cmds := make([]*redis.SliceCmd, len(spawns))
	for i, s := range spawns {
		cmds[i] = pipe.HMGet(ctx, monKey(pos.Zone, s.ID), "hp", "dead_until")
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return out, err
	}
	now := time.Now().UnixMilli()
	for i, s := range spawns {
		sp, _ := bestiary.Get(ws, s.Species)
		st := bestiary.StatsFor(sp, s.Level, s.Rank)
		ms := c.MonsterState{ID: s.ID, Spawn: s, HP: st.MaxHP, MaxHP: st.MaxHP}
		vals := cmds[i].Val()
		if len(vals) == 2 {
			dead := toInt(vals[1])
			if dead == -1 || dead > now {
				ms.DeadUntil, ms.HP = dead, 0
			} else if hp := toInt(vals[0]); vals[0] != nil && dead == 0 {
				ms.HP = int(hp)
			}
		}
		out.Monsters = append(out.Monsters, ms)
	}
	return out, nil
}

func toInt(v any) int64 {
	s, ok := v.(string)
	if !ok {
		return 0
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func (a *app) player(ctx context.Context, req c.CharacterReq) (c.PlayerVitals, error) {
	prof, err := a.profile(ctx, req.CharacterID)
	if err != nil {
		return c.PlayerVitals{}, err
	}
	hp, err := a.playerHP(ctx, req.CharacterID, prof.Derived.MaxHP)
	return c.PlayerVitals{HP: hp, MaxHP: prof.Derived.MaxHP}, err
}
