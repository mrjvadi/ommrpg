// history-service turns domain events into world history (MongoDB): a
// chronicle of notable events and permanent "world firsts" (first to reach a
// level, first legendary drop, first clear of each dungeon, ...), which are
// announced to everyone on the news channel.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/bestiary"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/items"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

func main() {
	s := svc.New("history")
	db, err := store.Mongo(s.Ctx, config.String("MONGO_URI", "mongodb://localhost:27017"), config.String("MONGO_DB", "ommrpg_history"))
	if err != nil {
		s.Fatal("mongo", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "history", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	if err := b.EnsureStreams(s.Ctx); err != nil {
		s.Fatal("streams", err)
	}
	a := &app{events: db.Collection("chronicle"), firsts: db.Collection("world_firsts"), log: s.Log,
		cf: centrifugo.New(config.String("CENTRIFUGO_API_URL", "http://localhost:8000"), config.String("CENTRIFUGO_API_KEY", "dev-api-key"))}
	_, _ = a.events.Indexes().CreateOne(s.Ctx, mongo.IndexModel{Keys: bson.D{{Key: "time", Value: -1}}})
	for _, err := range []error{
		bus.Handle(b, c.HistoryRecent, a.recent),
		bus.Handle(b, c.HistoryFirsts, a.worldFirsts),
		b.Consume(s.Ctx, "history", []string{"game.>"}, a.onEvent),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}

type app struct {
	events *mongo.Collection
	firsts *mongo.Collection
	cf     *centrifugo.Client
	log    *slog.Logger
}

type doc struct {
	ID      string         `bson:"_id"`
	Type    string         `bson:"type"`
	Time    time.Time      `bson:"time"`
	Summary string         `bson:"summary"`
	Data    map[string]any `bson:"data,omitempty"`
}

// describe decides whether an event is notable and how to phrase it, and
// which world-first keys it could claim.
func describe(ev bus.Event) (summary string, firsts []string, keep bool) {
	switch ev.Type {
	case c.EvCharacterCreated:
		d, _ := bus.Decode[c.CharacterCreatedEv](ev)
		return fmt.Sprintf("%s entered the world", d.Name), nil, true
	case c.EvCharacterLeveled:
		d, _ := bus.Decode[c.CharacterLeveledEv](ev)
		if d.Level%5 == 0 {
			return fmt.Sprintf("%s reached level %d", d.Name, d.Level), []string{fmt.Sprintf("level:%d", d.Level)}, true
		}
	case c.EvCharacterAwakened:
		d, _ := bus.Decode[c.CharacterAwakenedEv](ev)
		return fmt.Sprintf("%s awakened as a %s", d.Name, d.Class), []string{"class:" + d.Class}, true
	case c.EvMonsterKilled:
		d, _ := bus.Decode[c.MonsterKilledEv](ev)
		if d.Rank != bestiary.Normal {
			return fmt.Sprintf("the %s %s (lvl %d) was slain", d.Rank, d.SpeciesName, d.Level), []string{"slay:" + d.SpeciesName}, true
		}
	case c.EvItemDropped:
		d, _ := bus.Decode[c.ItemDroppedEv](ev)
		if d.Item.Rarity >= items.Epic {
			return fmt.Sprintf("a %s item appeared: %s", d.Item.Rarity, d.Item.Name),
				[]string{"drop:" + d.Item.Rarity.String(), "drop:" + d.Item.Rarity.String() + ":" + d.Item.Base}, true
		}
	case c.EvItemEnhanced:
		d, _ := bus.Decode[c.ItemEnhancedEv](ev)
		if d.Result.Success && d.Result.To >= 10 {
			return fmt.Sprintf("%s was enhanced to +%d", d.ItemName, d.Result.To), []string{fmt.Sprintf("enhance:+%d", d.Result.To)}, true
		}
	case c.EvDungeonCleared:
		d, _ := bus.Decode[c.DungeonClearedEv](ev)
		return fmt.Sprintf("%s (tier %d) was cleared", d.Name, d.Tier), []string{"clear:" + d.EntranceID}, true
	}
	return "", nil, false
}

func (a *app) onEvent(ctx context.Context, ev bus.Event) error {
	summary, firsts, keep := describe(ev)
	if !keep {
		return nil
	}
	var data map[string]any
	_ = bson.UnmarshalExtJSON(ev.Data, false, &data)
	_, err := a.events.InsertOne(ctx, doc{ID: ev.ID, Type: ev.Type, Time: ev.Time, Summary: summary, Data: data})
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return err
	}
	for _, key := range firsts {
		_, err := a.firsts.InsertOne(ctx, doc{ID: key, Type: ev.Type, Time: ev.Time, Summary: summary, Data: data})
		if mongo.IsDuplicateKeyError(err) {
			continue // someone got there first; history never changes
		}
		if err != nil {
			return err
		}
		_ = a.cf.Publish(ctx, centrifugo.NewsChannel, map[string]any{"t": "world_first", "key": key, "text": "World first! " + summary})
	}
	return nil
}

func (a *app) list(ctx context.Context, col *mongo.Collection, limit int) (c.HistoryResp, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	cur, err := col.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "time", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return c.HistoryResp{}, err
	}
	defer cur.Close(ctx)
	out := c.HistoryResp{Entries: []c.HistoryEntry{}}
	for cur.Next(ctx) {
		var d doc
		if err := cur.Decode(&d); err != nil {
			return out, err
		}
		out.Entries = append(out.Entries, c.HistoryEntry{ID: d.ID, Type: d.Type, Time: d.Time, Summary: d.Summary})
	}
	return out, errors.Join(cur.Err())
}

func (a *app) recent(ctx context.Context, req c.HistoryReq) (c.HistoryResp, error) {
	return a.list(ctx, a.events, req.Limit)
}

func (a *app) worldFirsts(ctx context.Context, req c.HistoryReq) (c.HistoryResp, error) {
	return a.list(ctx, a.firsts, req.Limit)
}
