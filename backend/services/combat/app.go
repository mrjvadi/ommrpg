package main

import (
	"context"
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
	"github.com/mrjvadi/ommrpg/backend/pkg/hot"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/metrics"
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
	stats    *metrics.Counters
}

func newApp(rdb *redis.Client, b *bus.Bus, z *zones.Resolver, cf *centrifugo.Client, secret uint64, m *metrics.Counters, log *slog.Logger) *app {
	return &app{rdb: rdb, bus: b, zones: z, cf: cf, secret: secret, log: log, stats: m, profiles: lru.New[string, c.CombatProfile](8192, 3*time.Second)}
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

func hpKey(id string) string { return "hp:" + id }

const regenPerSecond = 0.02 // fraction of max HP

// playerHP applies a delta (0 to just read) with lazy regeneration.
func (a *app) playerHP(ctx context.Context, id string, max, delta int) (int, bool, error) {
	return hot.Vitals(ctx, a.rdb, hpKey(id), max, delta, regenPerSecond, 24*3600)
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
	st := bestiary.StatsFor(species, spawn.Level, spawn.Rank)
	n, _ := a.rdb.Incr(ctx, "atkseq:"+req.CharacterID).Result()
	hit := a.resolveHit(seed.Derive(a.secret, "attack", req.CharacterID, n), prof, species, st)
	cdMs := int64(d.Cooldown * 1000 * 0.9) // small tolerance for latency jitter
	res, err := hot.Attack(ctx, a.rdb, monKey(pos.Zone, spawn.ID), "threat:"+pos.Zone+":"+spawn.ID, "cd:"+req.CharacterID,
		st.MaxHP, hit.Damage, int64(st.Respawn)*1000, 6*3600, req.CharacterID, cdMs)
	if err != nil {
		return c.AttackResp{}, err
	}
	switch res.State {
	case hot.OnCooldown:
		return c.AttackResp{}, apperr.New(apperr.Cooldown, "attack is on cooldown (%dms)", res.CooldownMs)
	case hot.AlreadyDead:
		return c.AttackResp{}, apperr.New(apperr.Conflict, "target is already dead")
	}
	a.stats.Inc("attacks", 1)
	heal := 0
	if d.LifeSteal > 0 {
		heal = int(float64(hit.Damage) * d.LifeSteal)
	}
	out := c.AttackResp{Hit: hit, TargetHP: int(res.HP), TargetMax: st.MaxHP}
	ax, ay := z.Area(mx, my)
	channel := z.Channel(ax, ay)
	counterDmg := 0
	if res.State == hot.Killed {
		out.Killed = true
		a.stats.Inc("kills", 1)
		a.rdb.ZIncrBy(ctx, "lb:kills", 1, req.CharacterID)
		shares := a.rewardParticipants(ctx, res, req.CharacterID, spawn, species, st, pos.Zone)
		if me, ok := shares[req.CharacterID]; ok {
			out.XP, out.Loot = me.XP, &me.Loot
		}
		_ = a.cf.Publish(ctx, channel, map[string]any{"t": "die", "id": spawn.ID, "until": res.DeadUntil, "by": req.CharacterID, "dmg": hit.Damage, "crit": hit.Crit, "el": hit.Element})
	} else {
		_ = a.cf.Publish(ctx, channel, map[string]any{"t": "hit", "id": spawn.ID, "hp": res.HP, "max": st.MaxHP, "by": req.CharacterID, "dmg": hit.Damage, "crit": hit.Crit, "el": hit.Element})
		// counterattack: the monster strikes back when the attacker is in its reach
		if combat.InRange(pos.X, pos.Y, mx, my, st.Range+0.5) {
			cd := time.Duration(st.Cooldown*1000) * time.Millisecond
			if ok, _ := a.rdb.SetNX(ctx, "mcd:"+pos.Zone+":"+spawn.ID+":"+req.CharacterID, 1, cd).Result(); ok {
				m, _ := a.rdb.Incr(ctx, "defseq:"+req.CharacterID).Result()
				counter := combat.Resolve(seed.Derive(a.secret, "counter", req.CharacterID, m), st.Attack, d.Defense, 0.05)
				out.Counter = &counter
				counterDmg = counter.Damage
			}
		}
	}
	php, died, err := a.playerHP(ctx, req.CharacterID, d.MaxHP, heal-counterDmg)
	if err != nil {
		return out, err
	}
	out.PlayerHP, out.PlayerMax = php, d.MaxHP
	if died {
		out.Died = true
		a.onDeath(ctx, req.CharacterID, prof, z, species.Name)
	}
	return out, nil
}

type share struct {
	XP   int64
	Loot items.Loot
}

// rewardParticipants splits a kill fairly: everyone who dealt at least 10%
// of the monster's HP (and always the killer) gets XP proportional to damage
// plus a group bonus (0.3 + 0.7*share), and their own personal loot roll
// seeded by the kill and the participant, so nobody can steal loot and a
// replayed event never pays twice.
func (a *app) rewardParticipants(ctx context.Context, res hot.AttackResult, killer string, spawn bestiary.Spawn, species bestiary.Species, st bestiary.Stats, zoneID string) map[string]share {
	var total int64
	for _, d := range res.Threat {
		total += d
	}
	out := map[string]share{}
	if total <= 0 {
		return out
	}
	for who, dmg := range res.Threat {
		frac := float64(dmg) / float64(total)
		if frac < 0.1 && who != killer {
			continue
		}
		prof, err := a.profile(ctx, who)
		if err != nil {
			continue
		}
		xp := float64(st.XP) * (0.3 + 0.7*frac) * prof.Hidden.XPMultiplier() * (1 + prof.Derived.XPBonus)
		if gap := prof.Level - spawn.Level; gap > 5 {
			xp *= math.Max(0.1, 1-0.15*float64(gap-5)) // farming weak monsters pays little
		}
		sh := share{XP: int64(math.Max(1, math.Round(xp)))}
		sh.Loot = items.RollLoot(seed.Derive(a.secret, "loot", zoneID, spawn.ID, res.DeadUntil, who), spawn.Level, string(spawn.Rank), prof.Hidden.Luck()+prof.Derived.MagicFind)
		out[who] = sh
		ev := c.MonsterKilledEv{
			CharacterID: who, MonsterID: spawn.ID, Species: species.ID, SpeciesName: species.Name,
			Rank: spawn.Rank, Level: spawn.Level, Zone: zoneID, XP: sh.XP, WeaponKind: prof.Derived.WeaponKind, Loot: sh.Loot,
			DamageShare: frac,
		}
		if err := a.bus.Publish(ctx, c.EvMonsterKilled, fmt.Sprintf("kill:%s:%s:%d:%s", zoneID, spawn.ID, res.DeadUntil, who), ev); err != nil {
			a.log.Error("publish kill", "err", err)
		}
	}
	return out
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
	hp, _, err := a.playerHP(ctx, req.CharacterID, prof.Derived.MaxHP, 0)
	return c.PlayerVitals{HP: hp, MaxHP: prof.Derived.MaxHP}, err
}

// resolveHit computes the damage of one attack (elements are added by the
// weapon effects system).
func (a *app) resolveHit(s uint64, prof c.CombatProfile, sp bestiary.Species, st bestiary.Stats) combat.Hit {
	d := prof.Derived
	return combat.ResolveElemental(s, d.Attack, st.Defense, d.CritChance, d.Element, d.ElementDmg, sp.Resist(d.Element))
}
