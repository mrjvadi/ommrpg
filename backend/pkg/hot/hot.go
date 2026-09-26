// Package hot holds the atomic hot-state algorithms that run *inside*
// Dragonfly as Lua scripts: rate limiting (GCRA), authoritative movement
// (distance token bucket + optimistic concurrency), area-of-interest
// queries, attacks with threat tables and exactly-once kills, player vitals
// with lazy regeneration, and sliding-window counters.
//
// Running them server-side makes every operation a single round trip and
// race-free across any number of service replicas. Scripts that touch keys
// they cannot declare up front carry Dragonfly's
// "--!df flags=allow-undeclared-keys" header, so the default (strict)
// Dragonfly configuration works.
package hot

import (
	"context"
	"embed"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/*.lua
var files embed.FS

func load(name string) *redis.Script {
	src, err := files.ReadFile("lua/" + name + ".lua")
	if err != nil {
		panic(err)
	}
	return redis.NewScript(string(src))
}

var (
	gcraScript    = load("gcra")
	moveScript    = load("move")
	placeScript   = load("place")
	nearbyScript  = load("nearby")
	attackScript  = load("attack")
	vitalsScript  = load("vitals")
	counterScript = load("counter")
)

// ---------------------------------------------------------------- rate limit

// Limit is the result of a GCRA check.
type Limit struct {
	Allowed    bool
	RetryAfter int64 // ms
	Remaining  int64
}

// Allow checks `cost` units against `limit` per `periodMs` with a burst.
func Allow(ctx context.Context, r redis.Scripter, key string, limit int, periodMs int64, burst, cost int) (Limit, error) {
	v, err := gcraScript.Run(ctx, r, []string{key}, periodMs, limit, burst, cost).Int64Slice()
	if err != nil {
		return Limit{Allowed: true}, err
	}
	return Limit{Allowed: v[0] == 1, RetryAfter: v[1], Remaining: v[2]}, nil
}

// ---------------------------------------------------------------- movement

// Pos is a character position as stored in the hot store.
type Pos struct {
	Zone string
	X, Y float64
	Dir  string
	Anim string
	At   int64
	Ver  int64
}

func f(s any) float64 {
	switch v := s.(type) {
	case string:
		x, _ := strconv.ParseFloat(v, 64)
		return x
	case int64:
		return float64(v)
	}
	return 0
}

// LoadPos reads a position hash; ok=false when absent.
func LoadPos(ctx context.Context, r redis.Cmdable, key string) (Pos, bool, error) {
	m, err := r.HGetAll(ctx, key).Result()
	if err != nil || len(m) == 0 || m["zone"] == "" {
		return Pos{}, false, err
	}
	at, _ := strconv.ParseInt(m["at"], 10, 64)
	ver, _ := strconv.ParseInt(m["ver"], 10, 64)
	return Pos{Zone: m["zone"], X: f(m["x"]), Y: f(m["y"]), Dir: m["dir"], Anim: m["anim"], At: at, Ver: ver}, true, nil
}

// MoveStatus of a Move call.
type MoveStatus int

const (
	MoveAccepted MoveStatus = 1
	MoveTooFast  MoveStatus = 0
	MoveConflict MoveStatus = -1
	MoveStale    MoveStatus = -2
)

// MoveArgs is one requested move.
type MoveArgs struct {
	PosKey, OldArea, NewArea, Member string
	ExpectedVer                      int64
	X, Y                             float64
	Dir, Anim                        string
	Speed, Cap                       float64
	Seq                              int64
	TTLSeconds                       int
}

// Move applies a move through the token bucket. It returns the status and the
// authoritative position after the call.
func Move(ctx context.Context, r redis.Scripter, a MoveArgs) (MoveStatus, int64, float64, float64, error) {
	v, err := moveScript.Run(ctx, r, []string{a.PosKey, a.OldArea, a.NewArea},
		a.ExpectedVer, fmtF(a.X), fmtF(a.Y), a.Dir, a.Anim, fmtF(a.Speed), fmtF(a.Cap), a.Seq, a.TTLSeconds, a.Member).Slice()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if len(v) < 4 {
		return 0, 0, 0, 0, fmt.Errorf("move: short reply")
	}
	return MoveStatus(v[0].(int64)), v[1].(int64), f(v[2]), f(v[3]), nil
}

// Place puts a character somewhere unconditionally and resets its bucket.
func Place(ctx context.Context, r redis.Scripter, posKey, oldArea, newArea, member string, p Pos, capTiles float64, ttl int) (int64, error) {
	if oldArea == "" {
		oldArea = newArea
	}
	return placeScript.Run(ctx, r, []string{posKey, oldArea, newArea},
		p.Zone, fmtF(p.X), fmtF(p.Y), p.Dir, p.Anim, fmtF(capTiles), ttl, member).Int64()
}

func fmtF(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }

// Nearby returns the live positions in the given area keys (one round trip).
func Nearby(ctx context.Context, r redis.Scripter, areaKeys []string, idleMs int64, self, zone, posPrefix string) ([]Pos, []string, error) {
	v, err := nearbyScript.Run(ctx, r, areaKeys, idleMs, self, zone, posPrefix).StringSlice()
	if err != nil {
		return nil, nil, err
	}
	var out []Pos
	var ids []string
	for i := 0; i+5 < len(v); i += 6 {
		at, _ := strconv.ParseInt(v[i+5], 10, 64)
		ids = append(ids, v[i])
		out = append(out, Pos{Zone: zone, X: f(v[i+1]), Y: f(v[i+2]), Dir: v[i+3], Anim: v[i+4], At: at})
	}
	return out, ids, nil
}

// ---------------------------------------------------------------- combat

// AttackState of an attack.
type AttackState int

const (
	Hit         AttackState = 0
	Killed      AttackState = 1
	AlreadyDead AttackState = 2
	OnCooldown  AttackState = 3
)

// AttackResult of one attack. Threat is set only when the monster died.
type AttackResult struct {
	State      AttackState
	HP         int64
	DeadUntil  int64
	CooldownMs int64
	Threat     map[string]int64
}

// Attack applies damage atomically (see attack.lua).
func Attack(ctx context.Context, r redis.Scripter, monKey, threatKey, cdKey string, maxHP, dmg int, respawnMs int64, ttl int, attacker string, cooldownMs int64) (AttackResult, error) {
	v, err := attackScript.Run(ctx, r, []string{monKey, threatKey, cdKey}, maxHP, dmg, respawnMs, ttl, attacker, cooldownMs).Slice()
	if err != nil {
		return AttackResult{}, err
	}
	res := AttackResult{State: AttackState(v[0].(int64)), HP: v[1].(int64), DeadUntil: v[2].(int64)}
	if res.State == OnCooldown {
		res.CooldownMs, res.HP = res.HP, 0
	}
	if res.State == Killed {
		res.Threat = map[string]int64{}
		for i := 3; i+1 < len(v); i += 2 {
			id, _ := v[i].(string)
			d, _ := strconv.ParseInt(fmt.Sprint(v[i+1]), 10, 64)
			res.Threat[id] = d
		}
	}
	return res, nil
}

// Vitals applies an HP delta with lazy regeneration; returns hp and died.
func Vitals(ctx context.Context, r redis.Scripter, key string, maxHP, delta int, regen float64, ttl int) (int, bool, error) {
	v, err := vitalsScript.Run(ctx, r, []string{key}, maxHP, delta, fmtF(regen), ttl).Int64Slice()
	if err != nil {
		return maxHP, false, err
	}
	return int(v[0]), v[1] == 1, nil
}

// Count adds n to a sliding-window counter and returns the window sum.
func Count(ctx context.Context, r redis.Scripter, key string, n int64, windowSec int) (int64, error) {
	v, err := counterScript.Run(ctx, r, []string{key}, n, windowSec).Int64Slice()
	if err != nil {
		return 0, err
	}
	return v[0], nil
}
