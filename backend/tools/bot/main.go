// Command bot is a headless game client. It logs in, creates a character,
// connects to Centrifugo and plays: walks (A* over the seed-generated map),
// fights monsters, collects loot, enhances gear and runs a dungeon. It is
// used as the end-to-end test of the whole backend and as a load generator
// (-n runs many bots in parallel).
//
//	go run ./tools/bot -api http://localhost:8080 -ws ws://localhost:8000/connection/websocket
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/centrifugal/centrifuge-go"

	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/dungeon"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
)

var (
	apiURL = flag.String("api", "http://localhost:8080", "gateway base url")
	wsURL  = flag.String("ws", "ws://localhost:8000/connection/websocket", "centrifugo websocket url")
	n      = flag.Int("n", 1, "number of bots")
	kills  = flag.Int("kills", 2, "monsters each bot should kill")
	doDng  = flag.Bool("dungeon", true, "also run a dungeon")
)

func main() {
	flag.Parse()
	var wg sync.WaitGroup
	fails := 0
	var mu sync.Mutex
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b := &bot{grid: map[[2]int]bool{}, name: fmt.Sprintf("bot%s", randHex(4)), log: log.New(os.Stdout, fmt.Sprintf("[bot%d] ", i), log.Ltime)}
			if err := b.run(); err != nil {
				b.log.Printf("FAIL: %v", err)
				mu.Lock()
				fails++
				mu.Unlock()
				return
			}
			b.log.Printf("PASS")
		}(i)
	}
	wg.Wait()
	if fails > 0 {
		os.Exit(1)
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type bot struct {
	name   string
	log    *log.Logger
	token  string
	char   c.Character
	world  c.WorldInfo
	pos    c.Position
	client *centrifuge.Client
	grid   map[[2]int]bool
	floor  *dungeon.Floor
	notes  chan map[string]any
}

// ---------------------------------------------------------------- http

func (b *bot) http(method, path string, body, out any) error {
	for attempt := 0; ; attempt++ {
		err := b.httpOnce(method, path, body, out)
		if err == nil || !strings.Contains(err.Error(), ": 429 ") || attempt >= 20 {
			return err
		}
		time.Sleep(150 * time.Millisecond) // rate limited: back off and retry
	}
}

func (b *bot) httpOnce(method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, *apiURL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if b.token != "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, raw)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (b *bot) rpc(method string, in, out any) error {
	raw, _ := json.Marshal(in)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := b.client.RPC(ctx, method, raw)
	if err != nil {
		return fmt.Errorf("rpc %s: %w", method, err)
	}
	if out != nil {
		return json.Unmarshal(res.Data, out)
	}
	return nil
}

// ---------------------------------------------------------------- flow

func (b *bot) run() error {
	var login struct {
		Token string `json:"token"`
	}
	if err := b.http("POST", "/api/v1/auth/dev", map[string]string{"username": b.name}, &login); err != nil {
		return err
	}
	b.token = login.Token
	if err := b.http("POST", "/api/v1/characters", map[string]string{"name": b.name}, &b.char); err != nil {
		return err
	}
	b.log.Printf("created %s (%s) body=%s layers=%d", b.char.Name, b.char.ID, b.char.Appearance.BodyType, len(b.char.Appearance.Layers))
	var sess struct {
		Token    string      `json:"token"`
		Position c.Position  `json:"position"`
		World    c.WorldInfo `json:"world"`
		Realtime struct {
			Token string `json:"token"`
		} `json:"realtime"`
	}
	if err := b.http("POST", "/api/v1/session/character", map[string]string{"character_id": b.char.ID}, &sess); err != nil {
		return err
	}
	b.token, b.pos, b.world = sess.Token, sess.Position, sess.World
	b.log.Printf("world %q seed=%d spawn=(%d,%d) pos=(%.1f,%.1f)", b.world.Name, b.world.Seed, b.world.SpawnX, b.world.SpawnY, b.pos.X, b.pos.Y)
	if err := b.connect(sess.Realtime.Token); err != nil {
		return err
	}
	defer b.client.Close()
	if err := b.rpc("enter", nil, &b.pos); err != nil {
		return err
	}
	if err := b.checkSprite(); err != nil {
		return err
	}
	var inv c.InventoryResp
	if err := b.waitFor(func() bool { _ = b.http("GET", "/api/v1/inventory", nil, &inv); return len(inv.Items) >= 2 }, 10*time.Second); err != nil {
		return fmt.Errorf("starter kit never arrived: %w", err)
	}
	b.log.Printf("starter kit: %d items, %d gold, %d essence", len(inv.Items), inv.Wallet.Gold, inv.Wallet.Essence)
	for k, tries := 0, 0; k < *kills; tries++ {
		if tries > *kills*5 {
			return errors.New("could not get enough kills")
		}
		err := b.huntOne()
		if errors.Is(err, errStolen) {
			continue // another player took it; find a new target
		}
		if err != nil {
			return fmt.Errorf("hunt %d: %w", k, err)
		}
		k++
	}
	if err := b.enhanceWeapon(); err != nil {
		return err
	}
	if *doDng {
		if err := b.runDungeon(); err != nil {
			return fmt.Errorf("dungeon: %w", err)
		}
	}
	var me c.Character
	if err := b.http("GET", "/api/v1/character", nil, &me); err != nil {
		return err
	}
	b.log.Printf("final: level %d xp %d/%d free points %d", me.Level, me.XP, me.XPToNext, me.FreePoints)
	if me.Level == 1 && me.XP == 0 {
		return errors.New("character gained no xp")
	}
	return nil
}

func (b *bot) connect(token string) error {
	b.notes = make(chan map[string]any, 64)
	b.client = centrifuge.NewJsonClient(*wsURL, centrifuge.Config{Token: token})
	b.client.OnPublication(func(e centrifuge.ServerPublicationEvent) {
		var m map[string]any
		if json.Unmarshal(e.Data, &m) == nil {
			select {
			case b.notes <- m:
			default:
			}
		}
	})
	connected := make(chan error, 1)
	b.client.OnConnected(func(centrifuge.ConnectedEvent) { connected <- nil })
	b.client.OnError(func(e centrifuge.ErrorEvent) { b.log.Printf("centrifugo error: %v", e.Error) })
	if err := b.client.Connect(); err != nil {
		return err
	}
	select {
	case err := <-connected:
		return err
	case <-time.After(5 * time.Second):
		return errors.New("realtime connect timeout")
	}
}

func (b *bot) waitFor(cond func() bool, d time.Duration) error {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if cond() {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("timeout")
}

func (b *bot) checkSprite() error {
	if os.Getenv("BOT_SKIP_SPRITE") != "" {
		return nil
	}
	var r struct {
		URL string `json:"url"`
	}
	if err := b.http("POST", "/api/v1/sprites/character", b.char.Appearance, &r); err != nil {
		return err
	}
	resp, err := http.Get(*apiURL + "/api" + r.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !bytes.HasPrefix(body, []byte("\x89PNG")) {
		return fmt.Errorf("sprite sheet: status %d (%d bytes)", resp.StatusCode, len(body))
	}
	b.log.Printf("sprite sheet ok (%d KB)", len(body)/1024)
	return nil
}

// ---------------------------------------------------------------- map & walking

func (b *bot) loadChunk(cx, cy int) error {
	var ch c.Chunk
	if err := b.http("GET", fmt.Sprintf("/api/v1/worlds/%d/chunks/%d/%d", b.world.ID, cx, cy), nil, &ch); err != nil {
		return err
	}
	for i := range ch.Terrain.Ground {
		x, y := cx*world.ChunkSize+i%world.ChunkSize, cy*world.ChunkSize+i/world.ChunkSize
		b.grid[[2]int{x, y}] = world.Ground(ch.Terrain.Ground[i]).Walkable() && !world.Object(ch.Terrain.Objects[i]).Blocking()
	}
	return nil
}

func (b *bot) walkable(x, y int) bool {
	if b.floor != nil {
		return b.floor.Walkable(x, y)
	}
	k := [2]int{x, y}
	if v, ok := b.grid[k]; ok {
		return v
	}
	cx, cy := world.ChunkOf(x, y)
	if cx < 0 || cy < 0 || cx >= b.world.SizeChunks || cy >= b.world.SizeChunks {
		return false
	}
	if err := b.loadChunk(cx, cy); err != nil {
		return false
	}
	return b.grid[k]
}

// path finds tiles from the current tile to any goal tile (A*, bounded).
func (b *bot) path(goal func(x, y int) bool, hx, hy int) [][2]int {
	type node struct{ x, y int }
	start := node{int(b.pos.X), int(b.pos.Y)}
	prev := map[node]node{start: start}
	g := map[node]int{start: 0}
	open := []node{start}
	for steps := 0; len(open) > 0 && steps < 40000; steps++ {
		bi := 0
		for i, o := range open {
			fo := g[o] + abs(o.x-hx) + abs(o.y-hy)
			fb := g[open[bi]] + abs(open[bi].x-hx) + abs(open[bi].y-hy)
			if fo < fb {
				bi = i
			}
		}
		cur := open[bi]
		open = append(open[:bi], open[bi+1:]...)
		if goal(cur.x, cur.y) {
			out := [][2]int{}
			for n := cur; n != start; n = prev[n] {
				out = append([][2]int{{n.x, n.y}}, out...)
			}
			return out
		}
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx := node{cur.x + d[0], cur.y + d[1]}
			if _, seen := prev[nx]; seen || !b.walkable(nx.x, nx.y) {
				continue
			}
			prev[nx] = cur
			g[nx] = g[cur] + 1
			open = append(open, nx)
		}
	}
	return nil
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// walk follows tile centres, sending moves at 10Hz within the speed limit.
func (b *bot) walk(tiles [][2]int) error {
	speed := 4.0 // a bit under the server's base speed
	for _, t := range tiles {
		tx, ty := float64(t[0])+0.5, float64(t[1])+0.5
		for {
			dx, dy := tx-b.pos.X, ty-b.pos.Y
			d := math.Hypot(dx, dy)
			if d < 0.05 {
				break
			}
			step := math.Min(d, speed*0.1)
			nx, ny := b.pos.X+dx/d*step, b.pos.Y+dy/d*step
			dir := "down"
			switch {
			case math.Abs(dx) > math.Abs(dy) && dx > 0:
				dir = "right"
			case math.Abs(dx) > math.Abs(dy):
				dir = "left"
			case dy < 0:
				dir = "up"
			}
			time.Sleep(100 * time.Millisecond)
			var mv c.MoveResp
			if err := b.rpc("move", c.MoveReq{X: nx, Y: ny, Dir: dir, Anim: "walk"}, &mv); err != nil {
				return err
			}
			b.pos = mv.Position
			if !mv.Accepted {
				return fmt.Errorf("move to (%.2f,%.2f) rejected; server says (%.2f,%.2f)", nx, ny, b.pos.X, b.pos.Y)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------- fighting

var errStolen = errors.New("target was killed by someone else")

func (b *bot) rest() error {
	for i := 0; i < 60; i++ {
		var v c.PlayerVitals
		if err := b.rpc("vitals", nil, &v); err != nil {
			return err
		}
		if v.HP*10 >= v.MaxHP*8 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (b *bot) huntOne() error {
	if err := b.rest(); err != nil {
		return err
	}
	var ms c.MonstersResp
	if err := b.rpc("monsters", nil, &ms); err != nil {
		return err
	}
	var target *c.MonsterState
	best := math.MaxFloat64
	for i, m := range ms.Monsters {
		d := math.Hypot(float64(m.Spawn.X)-b.pos.X, float64(m.Spawn.Y)-b.pos.Y)
		if m.DeadUntil == 0 && m.Spawn.Rank == "normal" && d < best {
			best, target = d, &ms.Monsters[i]
		}
	}
	if target == nil {
		return fmt.Errorf("no monster nearby (%d listed)", len(ms.Monsters))
	}
	b.log.Printf("hunting %s lvl %d at (%d,%d), %.0f tiles away", target.ID, target.Spawn.Level, target.Spawn.X, target.Spawn.Y, best)
	tx, ty := target.Spawn.X, target.Spawn.Y
	p := b.path(func(x, y int) bool { return abs(x-tx)+abs(y-ty) == 1 }, tx, ty)
	if p == nil {
		return errors.New("no path to monster")
	}
	if err := b.walk(p); err != nil {
		return err
	}
	for i := 0; i < 200; i++ {
		var res c.AttackResp
		err := b.rpc("attack", map[string]string{"target": target.ID}, &res)
		if err != nil {
			if strings.Contains(err.Error(), "cooldown") {
				time.Sleep(150 * time.Millisecond)
				continue
			}
			if strings.Contains(err.Error(), "already dead") {
				return errStolen
			}
			return err
		}
		if res.Died {
			return errors.New("bot died")
		}
		if res.Killed {
			if res.Loot == nil {
				return errors.New("killer received no reward")
			}
			b.log.Printf("killed %s: +%d xp, loot: %d gold, %d items (hp %d/%d)", target.ID, res.XP, res.Loot.Gold, len(res.Loot.Items), res.PlayerHP, res.PlayerMax)
			return b.expect("loot", 5*time.Second)
		}
		time.Sleep(700 * time.Millisecond)
	}
	return errors.New("monster never died")
}

func (b *bot) expect(kind string, d time.Duration) error {
	timeout := time.After(d)
	for {
		select {
		case m := <-b.notes:
			if m["t"] == kind {
				return nil
			}
		case <-timeout:
			return fmt.Errorf("no %q notification", kind)
		}
	}
}

func (b *bot) enhanceWeapon() error {
	var inv c.InventoryResp
	if err := b.http("GET", "/api/v1/inventory", nil, &inv); err != nil {
		return err
	}
	for _, it := range inv.Items {
		if it.Equipped != "weapon" {
			continue
		}
		rid := randHex(8)
		var r1, r2 c.EnhanceResp
		if err := b.http("POST", "/api/v1/inventory/"+it.ID+"/enhance", map[string]string{"request_id": rid}, &r1); err != nil {
			return err
		}
		// same request id again must be a no-op replay
		if err := b.http("POST", "/api/v1/inventory/"+it.ID+"/enhance", map[string]string{"request_id": rid}, &r2); err != nil {
			return err
		}
		if r1.Wallet != r2.Wallet || r1.Result != r2.Result {
			return errors.New("enhance is not idempotent")
		}
		if !r1.Result.Success {
			return errors.New("+0 -> +1 must always succeed")
		}
		b.log.Printf("enhanced %s to +%d (gold %d -> %d)", it.Item.Name, r1.Result.To, inv.Wallet.Gold, r1.Wallet.Gold)
		return nil
	}
	return errors.New("no equipped weapon")
}

// ---------------------------------------------------------------- dungeon

func (b *bot) runDungeon() error {
	sx, sy := world.ChunkOf(int(b.pos.X), int(b.pos.Y))
	var ent *world.Entrance
	best := math.MaxFloat64
	for r := 0; r <= 6 && ent == nil; r++ {
		for cy := sy - r; cy <= sy+r; cy++ {
			for cx := sx - r; cx <= sx+r; cx++ {
				if abs(cx-sx) != r && abs(cy-sy) != r {
					continue
				}
				var ch c.Chunk
				if b.http("GET", fmt.Sprintf("/api/v1/worlds/%d/chunks/%d/%d", b.world.ID, cx, cy), nil, &ch) != nil || ch.Entrance == nil {
					continue
				}
				if d := math.Hypot(float64(ch.Entrance.X)-b.pos.X, float64(ch.Entrance.Y)-b.pos.Y); d < best {
					best, ent = d, ch.Entrance
				}
			}
		}
	}
	if ent == nil {
		return errors.New("no dungeon entrance nearby")
	}
	b.log.Printf("heading to %q (tier %d) at (%d,%d), %.0f tiles", ent.Name, ent.Tier, ent.X, ent.Y, best)
	p := b.path(func(x, y int) bool { return abs(x-ent.X)+abs(y-ent.Y) == 1 }, ent.X, ent.Y)
	if p == nil {
		return errors.New("no path to entrance")
	}
	if err := b.walk(p); err != nil {
		return err
	}
	var in struct {
		Action  string        `json:"action"`
		Dungeon c.DungeonView `json:"dungeon"`
	}
	if err := b.rpc("interact", map[string]int{"x": ent.X, "y": ent.Y}, &in); err != nil {
		return err
	}
	b.floor, b.pos = in.Dungeon.Floor, in.Dungeon.Position
	b.log.Printf("entered %s: floor %d/%d %dx%d, %d monsters, %d chests", in.Dungeon.Name, b.floor.Index+1, b.floor.Floors, b.floor.W, b.floor.H, len(b.floor.Monsters), len(b.floor.Chests))
	var view c.DungeonView
	if err := b.rpc("dungeon", nil, &view); err != nil || view.InstanceID != in.Dungeon.InstanceID {
		return fmt.Errorf("dungeon view mismatch: %v", err)
	}
	ex := b.floor.Exit
	p = b.path(func(x, y int) bool { return abs(x-ex.X)+abs(y-ex.Y) == 1 }, ex.X, ex.Y)
	if p == nil {
		return errors.New("no path to exit")
	}
	if err := b.walk(p); err != nil {
		return err
	}
	var out struct {
		Position c.Position `json:"position"`
	}
	if err := b.rpc("interact", map[string]int{"x": ex.X, "y": ex.Y}, &out); err != nil {
		return err
	}
	b.floor, b.pos = nil, out.Position
	if !strings.HasPrefix(b.pos.Zone, "w:") || math.Hypot(b.pos.X-float64(ent.X), b.pos.Y-float64(ent.Y)) > 3 {
		return fmt.Errorf("not returned to the entrance: %+v", b.pos)
	}
	b.log.Printf("left the dungeon back at (%.1f,%.1f)", b.pos.X, b.pos.Y)
	return nil
}
