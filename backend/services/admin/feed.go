package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
	"github.com/mrjvadi/ommrpg/backend/pkg/metrics"
)

// FeedItem is one line of the live event feed.
type FeedItem struct {
	Kind    string    `json:"kind"` // event | audit
	Type    string    `json:"type"`
	Level   string    `json:"level"` // info | good | warn | money
	Text    string    `json:"text"`
	Time    time.Time `json:"time"`
	Subject string    `json:"subject,omitempty"` // character/account to open
}

const feedKey = "admin:feed:recent"

func ton(n int64) string { return fmt.Sprintf("%.3f TON", float64(n)/1e9) }

// summarize renders an event for the (Persian) admin panel.
func summarize(ev bus.Event) (FeedItem, bool) {
	it := FeedItem{Kind: "event", Type: ev.Type, Time: ev.Time, Level: "info"}
	switch ev.Type {
	case c.EvCharacterCreated:
		d, _ := bus.Decode[c.CharacterCreatedEv](ev)
		it.Text, it.Subject = fmt.Sprintf("کاراکتر تازه: %s", d.Name), d.CharacterID
	case c.EvCharacterLeveled:
		d, _ := bus.Decode[c.CharacterLeveledEv](ev)
		it.Text, it.Subject, it.Level = fmt.Sprintf("%s به سطح %d رسید", d.Name, d.Level), d.CharacterID, "good"
	case c.EvCharacterAwakened:
		d, _ := bus.Decode[c.CharacterAwakenedEv](ev)
		it.Text, it.Subject, it.Level = fmt.Sprintf("کلاس %s برای %s بیدار شد", d.Class, d.Name), d.CharacterID, "good"
	case c.EvCharacterDied:
		d, _ := bus.Decode[c.CharacterDiedEv](ev)
		it.Text, it.Subject, it.Level = fmt.Sprintf("یک بازیکن به دست %s کشته شد", d.KilledBy), d.CharacterID, "warn"
	case c.EvMonsterKilled:
		d, _ := bus.Decode[c.MonsterKilledEv](ev)
		if d.Rank == "normal" {
			return it, false // too frequent for the feed; counted in metrics
		}
		it.Text, it.Subject, it.Level = fmt.Sprintf("%s (%s، سطح %d) کشته شد", d.SpeciesName, d.Rank, d.Level), d.CharacterID, "good"
	case c.EvItemDropped:
		d, _ := bus.Decode[c.ItemDroppedEv](ev)
		if d.Item.Rarity < 3 {
			return it, false
		}
		it.Text, it.Subject, it.Level = fmt.Sprintf("آیتم %s افتاد: %s", d.Item.Rarity, d.Item.Name), d.CharacterID, "good"
	case c.EvItemEnhanced:
		d, _ := bus.Decode[c.ItemEnhancedEv](ev)
		if d.Result.To < 7 {
			return it, false
		}
		verb := "موفق"
		if !d.Result.Success {
			verb, it.Level = "ناموفق", "warn"
		}
		it.Text, it.Subject = fmt.Sprintf("ارتقای %s: %s (+%d)", d.ItemName, verb, d.Result.To), d.CharacterID
	case c.EvDungeonCleared:
		d, _ := bus.Decode[c.DungeonClearedEv](ev)
		it.Text, it.Subject, it.Level = fmt.Sprintf("سیاه‌چال «%s» (رده %d) پاک شد", d.Name, d.Tier), d.CharacterID, "good"
	case c.EvAssetMinted:
		d, _ := bus.Decode[c.AssetEv](ev)
		it.Text, it.Level = fmt.Sprintf("NFT #%d ساخته شد: %s", d.TokenID, d.Name), "money"
	case c.EvAssetListed:
		d, _ := bus.Decode[c.AssetEv](ev)
		it.Text, it.Level = fmt.Sprintf("NFT #%d برای فروش گذاشته شد: %s", d.TokenID, price(d.Currency, d.Price)), "money"
	case c.EvAssetSold:
		d, _ := bus.Decode[c.AssetEv](ev)
		it.Text, it.Level = fmt.Sprintf("NFT #%d فروخته شد: %s", d.TokenID, price(d.Currency, d.Price)), "money"
	case c.EvTonDeposit:
		d, _ := bus.Decode[c.TonEv](ev)
		it.Text, it.Subject, it.Level = fmt.Sprintf("واریز %s", ton(d.Amount)), d.Account, "money"
	case c.EvTonWithdrawn:
		d, _ := bus.Decode[c.TonEv](ev)
		it.Text, it.Subject, it.Level = fmt.Sprintf("برداشت %s: %s", ton(d.Amount), d.Status), d.Account, "money"
		if d.Status == "failed" {
			it.Level = "warn"
		}
	default:
		return it, false
	}
	return it, true
}

func price(cur string, p int64) string {
	if cur == "TON" {
		return ton(p)
	}
	return fmt.Sprintf("%d طلا", p)
}

func (a *app) onEvent(ctx context.Context, ev bus.Event) error {
	a.counter("events", 1)
	switch ev.Type {
	case c.EvAssetSold:
		if d, err := bus.Decode[c.AssetEv](ev); err == nil && d.Currency == "TON" {
			a.counter("ton_volume", d.Price)
		}
	}
	it, ok := summarize(ev)
	if !ok {
		return nil
	}
	raw, _ := json.Marshal(it)
	a.rdb.LPush(ctx, feedKey, raw)
	a.rdb.LTrim(ctx, feedKey, 0, 199)
	return a.cf.Publish(ctx, "admin:feed", it)
}

func (a *app) counter(name string, n int64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = hotCount(ctx, a, name, n)
}

// ---------------------------------------------------------------- metrics

// Snapshot is one point of the live dashboard (every 2 seconds).
type Snapshot struct {
	T             int64             `json:"t"`
	Online        int               `json:"online"`
	Zones         map[string]int    `json:"zones,omitempty"`
	KillsPerMin   int64             `json:"kills_per_min"`
	AttacksPerMin int64             `json:"attacks_per_min"`
	MovesPerSec   float64           `json:"moves_per_sec"`
	CheatsPerMin  int64             `json:"rejected_moves_per_min"`
	EventsPerMin  int64             `json:"events_per_min"`
	TonVolume1h   int64             `json:"ton_volume_1h"`
	Asset         *c.AssetStatsResp `json:"asset,omitempty"`
	Services      map[string]bool   `json:"services,omitempty"`
}

type ring struct {
	mu  sync.Mutex
	buf []Snapshot
	max int
}

func newRing(n int) *ring { return &ring{max: n} }

func (r *ring) add(s Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, s)
	if len(r.buf) > r.max {
		r.buf = r.buf[len(r.buf)-r.max:]
	}
}

func (r *ring) all() []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Snapshot(nil), r.buf...)
}

// probes: one cheap request per service; any reply except "unavailable"
// (no responders / timeout) means the service is up.
var probes = map[string]struct {
	subject string
	body    any
}{
	"identity":  {c.IdentityGet, c.AccountReq{}},
	"character": {c.CharacterTop, c.SearchReq{Limit: 1}},
	"world":     {c.WorldDefault, struct{}{}},
	"item":      {c.ItemSnapshot, c.VaultReq{ItemID: "00000000-0000-0000-0000-000000000000"}},
	"asset":     {c.ChainInfo, struct{}{}},
	"ton":       {c.TonInfo, struct{}{}},
	"presence":  {c.PresenceStatsSubject, struct{}{}},
	"combat":    {c.CombatPlayer, c.CharacterReq{}},
	"dungeon":   {c.DungeonInstance, c.DungeonReq{InstanceID: "probe"}},
	"history":   {c.HistoryRecent, c.HistoryReq{Limit: 1}},
	"sprite":    {c.SpriteRandom, c.SpriteRandomReq{}},
}

func (a *app) probe(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, p := range probes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_, err := bus.Request[json.RawMessage](pctx, a.bus, p.subject, p.body)
			mu.Lock()
			out[name] = err == nil || !apperr.Is(err, apperr.Unavailable)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (a *app) metricsLoop(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	var asset *c.AssetStatsResp
	var services map[string]bool
	for i := 0; ; i++ {
		if i%5 == 0 { // every 10s: slower, heavier probes
			if st, err := bus.Request[c.AssetStatsResp](ctx, a.bus, c.AssetStats, struct{}{}); err == nil {
				asset = &st
			}
			services = a.probe(ctx)
		}
		s := Snapshot{T: time.Now().UnixMilli(), Asset: asset, Services: services}
		if ps, err := bus.Request[c.PresenceStats](ctx, a.bus, c.PresenceStatsSubject, struct{}{}); err == nil {
			s.Online, s.Zones = ps.Online, ps.Zones
		}
		s.KillsPerMin = metrics.Rate(ctx, a.rdb, "kills", 60)
		s.AttacksPerMin = metrics.Rate(ctx, a.rdb, "attacks", 60)
		s.MovesPerSec = float64(metrics.Rate(ctx, a.rdb, "moves", 10)) / 10
		s.CheatsPerMin = metrics.Rate(ctx, a.rdb, "move_rejected", 60)
		s.EventsPerMin = metrics.Rate(ctx, a.rdb, "events", 60)
		s.TonVolume1h = metrics.Rate(ctx, a.rdb, "ton_volume", metrics.Window)
		a.history.add(s)
		_ = a.cf.Publish(ctx, "admin:metrics", s)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *app) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	feed := []FeedItem{}
	for _, raw := range a.rdb.LRange(ctx, feedKey, 0, 99).Val() {
		var it FeedItem
		if json.Unmarshal([]byte(raw), &it) == nil {
			feed = append(feed, it)
		}
	}
	top, _ := bus.Request[c.CharacterListResp](ctx, a.bus, c.CharacterTop, c.SearchReq{Limit: 10})
	wd, _ := bus.Request[c.WorldInfo](ctx, a.bus, c.WorldDefault, struct{}{})
	httpx.JSON(w, 200, map[string]any{"history": a.history.all(), "feed": feed, "top": top.Characters, "world": wd})
}
