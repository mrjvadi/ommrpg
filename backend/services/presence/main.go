// presence-service is the authority on where every character is. It
// validates movement (speed + collision against the seed-generated map),
// keeps positions in Dragonfly and fans movement out to area channels in
// Centrifugo so nearby players see each other.
package main

import (
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

func main() {
	s := svc.New("presence")
	rdb, err := store.Dragonfly(s.Ctx, config.String("DRAGONFLY_URL", "redis://localhost:6379/0"))
	if err != nil {
		s.Fatal("dragonfly", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "presence", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	a := newApp(rdb, b, zones.New(b), centrifugo.New(config.String("CENTRIFUGO_API_URL", "http://localhost:8000"), config.String("CENTRIFUGO_API_KEY", "dev-api-key")), s.Log)
	for _, err := range []error{
		bus.Handle(b, c.PresenceEnter, a.enter),
		bus.Handle(b, c.PresenceMove, a.move),
		bus.Handle(b, c.PresenceGet, a.get),
		bus.Handle(b, c.PresenceNearby, a.nearby),
		bus.Handle(b, c.PresenceTeleport, a.teleport),
		bus.Handle(b, c.PresenceLook, a.look),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}
