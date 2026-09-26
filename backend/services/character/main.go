// character-service owns characters: hidden Root/Talent, attributes, level,
// XP, class awakening, appearance recipe and last persisted location.
package main

import (
	"embed"
	"io/fs"

	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	s := svc.New("character")
	db, err := store.Postgres(s.Ctx, config.String("POSTGRES_DSN", "postgres://ommrpg:ommrpg@localhost:5432/character?sslmode=disable"))
	if err != nil {
		s.Fatal("postgres", err)
	}
	sub, _ := fs.Sub(migrations, "migrations")
	if err := store.Migrate(s.Ctx, db, sub, "character"); err != nil {
		s.Fatal("migrate", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "character", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	if err := b.EnsureStreams(s.Ctx); err != nil {
		s.Fatal("streams", err)
	}
	a := newApp(db, b, centrifugo.New(config.String("CENTRIFUGO_API_URL", "http://localhost:8000"), config.String("CENTRIFUGO_API_KEY", "dev-api-key")), s.Log)
	for _, err := range []error{
		bus.Handle(b, c.CharacterCreate, a.create),
		bus.Handle(b, c.CharacterList, a.list),
		bus.Handle(b, c.CharacterGet, a.get),
		bus.Handle(b, c.CharacterPublic, a.public),
		bus.Handle(b, c.CharacterAllocate, a.allocate),
		bus.Handle(b, c.CharacterCombatProfile, a.combatProfile),
		bus.Handle(b, c.CharacterLocationGet, a.locationGet),
		bus.Handle(b, c.CharacterLocationSet, a.locationSet),
		bus.Handle(b, c.CharacterSearch, a.search),
		bus.Handle(b, c.CharacterTop, a.top),
		b.Consume(s.Ctx, "character", []string{c.EvMonsterKilled, c.EvItemEnhanced, c.EvItemSalvaged, c.EvDungeonCleared, c.EvCharacterDied}, a.onEvent),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}
