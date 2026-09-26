package hot

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Tests run against a real Dragonfly (or Redis): HOT_TEST_URL, default
// redis://localhost:6380/0. They are skipped when nothing is listening.
func client(t *testing.T) *redis.Client {
	url := os.Getenv("HOT_TEST_URL")
	if url == "" {
		url = "redis://localhost:6380/0"
	}
	opt, _ := redis.ParseURL(url)
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Skipf("no dragonfly at %s: %v", url, err)
	}
	return c
}

func uniq(p string) string { return fmt.Sprintf("test:%s:%d", p, time.Now().UnixNano()) }

func TestGCRA(t *testing.T) {
	c, ctx := client(t), context.Background()
	key := uniq("gcra")
	allowed := 0
	for i := 0; i < 30; i++ {
		l, err := Allow(ctx, c, key, 10, 1000, 5, 1)
		if err != nil {
			t.Fatal(err)
		}
		if l.Allowed {
			allowed++
		} else if l.RetryAfter <= 0 {
			t.Fatal("rejection must carry retry-after")
		}
	}
	if allowed < 5 || allowed > 7 {
		t.Fatalf("burst of 5 expected, allowed %d", allowed)
	}
	time.Sleep(250 * time.Millisecond) // 10/s -> ~2 more tokens
	l, _ := Allow(ctx, c, key, 10, 1000, 5, 1)
	if !l.Allowed {
		t.Fatal("limiter did not refill")
	}
}

func TestMoveBucketAndConflicts(t *testing.T) {
	c, ctx := client(t), context.Background()
	pk, a1, a2 := uniq("pos"), uniq("area1"), uniq("area2")
	ver, err := Place(ctx, c, pk, "", a1, "me", Pos{Zone: "w:1", X: 10, Y: 10, Dir: "down", Anim: "idle"}, 2, 60)
	if err != nil || ver != 1 {
		t.Fatalf("place: %v %d", err, ver)
	}
	base := MoveArgs{PosKey: pk, OldArea: a1, NewArea: a1, Member: "me", Dir: "right", Anim: "walk", Speed: 4, Cap: 2, TTLSeconds: 60}
	// within the initial bucket (2 tiles)
	m := base
	m.ExpectedVer, m.X, m.Y, m.Seq = 1, 11.5, 10, 1
	st, ver, x, _, err := Move(ctx, c, m)
	if err != nil || st != MoveAccepted || ver != 2 || x != 11.5 {
		t.Fatalf("move1: %v %v %v %v", st, ver, x, err)
	}
	// a teleport-sized jump is rejected, position unchanged
	m.ExpectedVer, m.X, m.Seq = 2, 30, 2
	st, _, x, _, _ = Move(ctx, c, m)
	if st != MoveTooFast || x != 11.5 {
		t.Fatalf("speed hack accepted: %v %v", st, x)
	}
	// stale version -> conflict
	m.ExpectedVer, m.X, m.Seq = 1, 12, 3
	if st, _, _, _, _ = Move(ctx, c, m); st != MoveConflict {
		t.Fatalf("expected conflict, got %v", st)
	}
	// out-of-order sequence -> stale
	m.ExpectedVer, m.X, m.Seq = 2, 12, 1
	if st, _, _, _, _ = Move(ctx, c, m); st != MoveStale {
		t.Fatalf("expected stale, got %v", st)
	}
	// crossing areas moves the membership
	m.ExpectedVer, m.X, m.Seq, m.NewArea = 2, 12, 4, a2
	if st, _, _, _, _ = Move(ctx, c, m); st != MoveAccepted {
		t.Fatalf("area move: %v", st)
	}
	if n, _ := c.ZCard(ctx, a1).Result(); n != 0 {
		t.Fatal("old area still lists the player")
	}
}

func TestNearby(t *testing.T) {
	c, ctx := client(t), context.Background()
	prefix := uniq("p") + ":"
	area := uniq("area")
	for i, id := range []string{"a", "b", "c"} {
		_, err := Place(ctx, c, prefix+id, "", area, id, Pos{Zone: "w:1", X: float64(i), Y: 1, Dir: "down", Anim: "idle"}, 2, 60)
		if err != nil {
			t.Fatal(err)
		}
	}
	ps, ids, err := Nearby(ctx, c, []string{area}, 60000, "a", "w:1", prefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || len(ids) != 2 {
		t.Fatalf("expected 2 others, got %v", ids)
	}
}

func TestAttackExactlyOnceKillAndThreat(t *testing.T) {
	c, ctx := client(t), context.Background()
	mk, tk := uniq("mon"), uniq("threat")
	var mu sync.Mutex
	kills := 0
	var threat map[string]int64
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			who := fmt.Sprintf("p%d", i%4)
			r, err := Attack(ctx, c, mk, tk, uniq("cd"+who), 100, 7, 60000, 60, who, 1)
			if err != nil {
				t.Error(err)
				return
			}
			if r.State == Killed {
				mu.Lock()
				kills++
				threat = r.Threat
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if kills != 1 {
		t.Fatalf("expected exactly one kill, got %d", kills)
	}
	total := int64(0)
	for _, d := range threat {
		total += d
	}
	if total != 100 {
		t.Fatalf("threat must sum to max hp (overkill capped), got %d %v", total, threat)
	}
	r, _ := Attack(ctx, c, mk, tk, uniq("cd"), 100, 7, 60000, 60, "late", 1)
	if r.State != AlreadyDead || r.DeadUntil == 0 {
		t.Fatalf("dead monster must stay dead: %+v", r)
	}
	// cooldown
	cd := uniq("cd")
	mk2 := uniq("mon")
	if r, _ := Attack(ctx, c, mk2, uniq("t"), cd, 100, 1, 1000, 60, "x", 5000); r.State != Hit {
		t.Fatal("first hit")
	}
	if r, _ := Attack(ctx, c, mk2, uniq("t"), cd, 100, 1, 1000, 60, "x", 5000); r.State != OnCooldown || r.CooldownMs <= 0 {
		t.Fatalf("cooldown not enforced: %+v", r)
	}
}

func TestVitalsAndCounter(t *testing.T) {
	c, ctx := client(t), context.Background()
	k := uniq("hp")
	hp, died, err := Vitals(ctx, c, k, 100, -30, 0.02, 60)
	if err != nil || hp != 70 || died {
		t.Fatalf("damage: %d %v %v", hp, died, err)
	}
	time.Sleep(600 * time.Millisecond)
	hp, _, _ = Vitals(ctx, c, k, 100, 0, 0.02, 60)
	if hp < 70 || hp > 72 {
		t.Fatalf("regen off: %d", hp)
	}
	hp, died, _ = Vitals(ctx, c, k, 100, -500, 0.02, 60)
	if !died || hp != 100 {
		t.Fatalf("death must reset to max: %d %v", hp, died)
	}
	ck := uniq("cnt")
	for i := 0; i < 5; i++ {
		_, _ = Count(ctx, c, ck, 2, 60)
	}
	if s, _ := Count(ctx, c, ck, 0, 60); s != 10 {
		t.Fatalf("window sum %d", s)
	}
}
