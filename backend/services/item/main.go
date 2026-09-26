// item-service owns items, inventories, currencies and the economic ledger.
// It generates items from seeds, grants loot, levels items through use and
// performs enhancement (+N upgrades) and salvage transactionally.
package main

import (
	"embed"
	"io/fs"

	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	s := svc.New("item")
	db, err := store.Postgres(s.Ctx, config.String("POSTGRES_DSN", "postgres://ommrpg:ommrpg@localhost:5432/item?sslmode=disable"))
	if err != nil {
		s.Fatal("postgres", err)
	}
	sub, _ := fs.Sub(migrations, "migrations")
	if err := store.Migrate(s.Ctx, db, sub, "item"); err != nil {
		s.Fatal("migrate", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "item", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	if err := b.EnsureStreams(s.Ctx); err != nil {
		s.Fatal("streams", err)
	}
	a := &app{
		db: db, bus: b, log: s.Log,
		cf:     centrifugo.New(config.String("CENTRIFUGO_API_URL", "http://localhost:8000"), config.String("CENTRIFUGO_API_KEY", "dev-api-key")),
		secret: seed.FromString(config.String("GAME_SECRET", "dev-game-secret")),
		maxInv: config.Int("INVENTORY_SIZE", 60),
	}
	for _, err := range []error{
		bus.Handle(b, c.ItemList, a.list),
		bus.Handle(b, c.ItemEquip, a.equip),
		bus.Handle(b, c.ItemUnequip, a.unequip),
		bus.Handle(b, c.ItemEnhance, a.enhance),
		bus.Handle(b, c.ItemSalvage, a.salvage),
		bus.Handle(b, c.ItemBonuses, a.bonuses),
		bus.Handle(b, c.ItemVault, a.vault),
		bus.Handle(b, c.ItemUnvault, a.unvault),
		bus.Handle(b, c.ItemClaim, a.claim),
		bus.Handle(b, c.ItemSnapshot, a.snapshot),
		bus.Handle(b, c.ItemGoldTransfer, a.goldTransfer),
		bus.Handle(b, c.ItemAdminGrant, a.adminGrant),
		b.Consume(s.Ctx, "item", []string{c.EvCharacterCreated, c.EvMonsterKilled, c.EvChestOpened}, a.onEvent),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}
