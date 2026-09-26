// combat-service is the authoritative battle engine: it validates range and
// cooldowns, resolves damage with audited seeded rolls, keeps monster and
// player vitals in Dragonfly and emits kill events with loot.
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
	s := svc.New("combat")
	rdb, err := store.Dragonfly(s.Ctx, config.String("DRAGONFLY_URL", "redis://localhost:6379/0"))
	if err != nil {
		s.Fatal("dragonfly", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "combat", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	if err := b.EnsureStreams(s.Ctx); err != nil {
		s.Fatal("streams", err)
	}
	a := newApp(rdb, b, zones.New(b),
		centrifugo.New(config.String("CENTRIFUGO_API_URL", "http://localhost:8000"), config.String("CENTRIFUGO_API_KEY", "dev-api-key")),
		seed.FromString(config.String("GAME_SECRET", "dev-game-secret")), s.Log)
	for _, err := range []error{
		bus.Handle(b, c.CombatAttack, a.attack),
		bus.Handle(b, c.CombatMonsters, a.monsters),
		bus.Handle(b, c.CombatPlayer, a.player),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}
