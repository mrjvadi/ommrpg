// asset-service owns tokenised items (NFTs) on OMM Chain, the marketplace
// (TON and gold listings), custodial TON balances with an append-only
// ledger, withdrawals, and the chain itself (signed Merkle blocks).
//
// Cross-service operations (mint, claim, gold purchases) are sagas: the
// token row records the in-flight op id; item-service steps are idempotent
// on that id, and a reconciler finishes or rolls back anything interrupted.
package main

import (
	"embed"
	"io/fs"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/chain"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	s := svc.New("asset")
	db, err := store.Postgres(s.Ctx, config.String("POSTGRES_DSN", "postgres://ommrpg:ommrpg@localhost:5432/asset?sslmode=disable"))
	if err != nil {
		s.Fatal("postgres", err)
	}
	sub, _ := fs.Sub(migrations, "migrations")
	if err := store.Migrate(s.Ctx, db, sub, "asset"); err != nil {
		s.Fatal("migrate", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "asset", s.Log)
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
		signer: chain.NewSigner(config.String("CHAIN_SIGNING_KEY", config.String("GAME_SECRET", "dev-game-secret"))),
		policy: lru.New[string, c.MarketPolicy](1, 5*time.Second),
	}
	if err := a.ensureGenesis(s.Ctx); err != nil {
		s.Fatal("genesis", err)
	}
	for _, err := range []error{
		bus.Handle(b, c.AssetMint, a.mint),
		bus.Handle(b, c.AssetClaim, a.claim),
		bus.Handle(b, c.AssetDeposit, a.depositToken),
		bus.Handle(b, c.AssetBurn, a.burn),
		bus.Handle(b, c.AssetTokens, a.tokens),
		bus.Handle(b, c.AssetToken, a.token),
		bus.Handle(b, c.AssetList, a.list),
		bus.Handle(b, c.AssetCancel, a.cancel),
		bus.Handle(b, c.AssetAdminCancel, a.adminCancel),
		bus.Handle(b, c.AssetMarket, a.market),
		bus.Handle(b, c.AssetBuy, a.buy),
		bus.Handle(b, c.AssetWallet, a.wallet),
		bus.Handle(b, c.AssetWithdraw, a.withdraw),
		bus.Handle(b, c.AssetTonCredit, a.tonCredit),
		bus.Handle(b, c.AssetWithdrawals, a.withdrawals),
		bus.Handle(b, c.AssetWithdrawalReview, a.review),
		bus.Handle(b, c.AssetWithdrawalResult, a.result),
		bus.Handle(b, c.AssetPolicyGet, a.policyGet),
		bus.Handle(b, c.AssetPolicySet, a.policySet),
		bus.Handle(b, c.AssetStats, a.stats),
		bus.Handle(b, c.ChainInfo, a.chainInfo),
		bus.Handle(b, c.ChainBlocks, a.blocks),
		bus.Handle(b, c.ChainBlock, a.block),
		bus.Handle(b, c.ChainProof, a.proof),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	go a.blockProducer(s.Ctx, config.Duration("CHAIN_BLOCK_INTERVAL", 3*time.Second))
	go a.reconciler(s.Ctx)
	s.Ready()
	s.Wait()
}
