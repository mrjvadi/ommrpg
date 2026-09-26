package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

type worker struct {
	bus   *bus.Bus
	rdb   *redis.Client
	chain Chain
	log   *slog.Logger
}

const cursorKey = "ton:deposit_cursor"

// watchDeposits polls the wallet and forwards transfers to asset-service.
// The cursor (last seen logical time) only advances after asset-service
// accepted the deposit; crediting is idempotent on the tx hash anyway.
func (w *worker) watchDeposits(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		cursor, _ := strconv.ParseUint(w.rdb.Get(ctx, cursorKey).Val(), 10, 64)
		deps, err := w.chain.Deposits(ctx, cursor)
		if err != nil {
			w.log.Warn("poll deposits", "err", err)
		}
		for _, d := range deps {
			_, err := bus.Request[struct{}](ctx, w.bus, c.AssetTonCredit, c.TonCreditReq{TxHash: d.TxHash, LT: d.LT, Amount: d.Amount, Memo: d.Memo, Sender: d.Sender})
			if err != nil {
				w.log.Warn("credit deposit", "tx", d.TxHash, "err", err)
				break // retry from here next round
			}
			w.rdb.Set(ctx, cursorKey, strconv.FormatUint(d.LT, 10), 0)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sendWithdrawals sends approved withdrawals one by one:
// approved -> sending (persisted BEFORE broadcasting) -> sent | failed.
func (w *worker) sendWithdrawals(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		list, err := bus.Request[c.WithdrawalsResp](ctx, w.bus, c.AssetWithdrawals, c.WithdrawalsReq{Status: "approved", Limit: 20})
		if err != nil {
			continue
		}
		for _, wd := range list.Withdrawals {
			if _, err := bus.Request[c.Withdrawal](ctx, w.bus, c.AssetWithdrawalResult, c.WithdrawalResult{ID: wd.ID, Status: "sending"}); err != nil {
				continue // someone else took it or it changed
			}
			hash, err := w.chain.Send(ctx, wd.To, wd.Amount-wd.Fee, fmt.Sprintf("OMMRPG withdrawal #%d", wd.ID))
			res := c.WithdrawalResult{ID: wd.ID, Status: "sent", TxHash: hash}
			if err != nil {
				res = c.WithdrawalResult{ID: wd.ID, Status: "failed", Note: err.Error()}
			}
			for i := 0; i < 5; i++ {
				if _, err := bus.Request[c.Withdrawal](ctx, w.bus, c.AssetWithdrawalResult, res); err == nil {
					break
				}
				time.Sleep(time.Second)
			}
			w.log.Info("withdrawal processed", "id", wd.ID, "status", res.Status, "tx", hash)
		}
	}
}
