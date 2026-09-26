// ton-service is the bridge between the game's custodial TON ledger
// (asset-service) and the TON blockchain:
//
//   - watches the game wallet for incoming transfers and credits accounts by
//     the transfer comment (each account has a unique deposit memo);
//   - sends approved withdrawals from the game wallet.
//
// TON_MODE=mock (default) simulates the chain for development and tests;
// TON_MODE=mainnet|testnet connects to lite servers with tonutils-go.
package main

import (
	"context"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

// Deposit is an incoming transfer found on chain.
type Deposit struct {
	TxHash string
	LT     uint64
	Amount int64
	Memo   string
	Sender string
}

// Chain abstracts the TON network.
type Chain interface {
	Address() string
	Network() string
	// Deposits returns incoming transfers newer than afterLT (oldest first).
	Deposits(ctx context.Context, afterLT uint64) ([]Deposit, error)
	// Send transfers nanoTON with a comment and returns the tx hash.
	Send(ctx context.Context, to string, nano int64, comment string) (string, error)
}

func main() {
	s := svc.New("ton")
	rdb, err := store.Dragonfly(s.Ctx, config.String("DRAGONFLY_URL", "redis://localhost:6379/0"))
	if err != nil {
		s.Fatal("dragonfly", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "ton", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	var ch Chain
	mode := config.String("TON_MODE", "mock")
	switch mode {
	case "mock":
		m := newMock()
		ch = m
		if err := bus.Handle(b, c.TonMockCredit, m.inject); err != nil {
			s.Fatal("subscribe", err)
		}
	case "mainnet", "testnet":
		ch, err = newLite(s.Ctx, mode, config.String("TON_WALLET_SEED", ""), config.String("TON_WALLET_VERSION", "v4r2"), config.String("TON_CONFIG_URL", ""))
		if err != nil {
			s.Fatal("ton wallet", err)
		}
	default:
		s.Fatal("config", errBadMode(mode))
	}
	s.Log.Info("ton wallet ready", "mode", mode, "address", ch.Address())
	if err := bus.Handle(b, c.TonInfo, func(context.Context, struct{}) (c.TonInfoResp, error) {
		return c.TonInfoResp{Address: ch.Address(), Network: ch.Network()}, nil
	}); err != nil {
		s.Fatal("subscribe", err)
	}
	w := &worker{bus: b, rdb: rdb, chain: ch, log: s.Log}
	go w.watchDeposits(s.Ctx, config.Duration("TON_POLL", 10*time.Second))
	go w.sendWithdrawals(s.Ctx, config.Duration("TON_SEND_EVERY", 10*time.Second))
	s.Ready()
	s.Wait()
}

type errBadMode string

func (e errBadMode) Error() string { return "unknown TON_MODE " + string(e) }
