// dungeon-service runs dungeon instances. Every overworld entrance has a
// fixed seed (so its layout is learnable), while each run is a fresh
// instance with its own monster and chest state. Leaving is return-bound:
// the server remembers the entrance and always sends you back there.
package main

import (
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

func main() {
	s := svc.New("dungeon")
	rdb, err := store.Dragonfly(s.Ctx, config.String("DRAGONFLY_URL", "redis://localhost:6379/0"))
	if err != nil {
		s.Fatal("dragonfly", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "dungeon", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	if err := b.EnsureStreams(s.Ctx); err != nil {
		s.Fatal("streams", err)
	}
	a := &app{rdb: rdb, bus: b, zones: zones.New(b), log: s.Log,
		cf:     centrifugo.New(config.String("CENTRIFUGO_API_URL", "http://localhost:8000"), config.String("CENTRIFUGO_API_KEY", "dev-api-key")),
		secret: seed.FromString(config.String("GAME_SECRET", "dev-game-secret"))}
	for _, err := range []error{
		bus.Handle(b, c.DungeonEnter, a.enter),
		bus.Handle(b, c.DungeonFloor, a.floor),
		bus.Handle(b, c.DungeonDescend, a.descend),
		bus.Handle(b, c.DungeonLeave, a.leave),
		bus.Handle(b, c.DungeonInstance, a.instance),
		bus.Handle(b, c.DungeonOpenChest, a.openChest),
		b.Consume(s.Ctx, "dungeon", []string{c.EvMonsterKilled}, a.onKill),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}
