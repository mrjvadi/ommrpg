package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/xssnick/tonutils-go/address"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

// tonString formats nanoTON as a decimal TON string.
func tonString(nano int64) string {
	s := strconv.FormatInt(nano/1_000_000_000, 10)
	frac := nano % 1_000_000_000
	if frac == 0 {
		return s
	}
	return s + "." + strings.TrimRight(fmt.Sprintf("%09d", frac), "0")
}

// moveTon changes a TON balance and writes the ledger (inside tx).
func moveTon(ctx context.Context, tx pgx.Tx, account string, delta int64, reason, ref string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ton_balances (account_id) VALUES ($1) ON CONFLICT DO NOTHING`, account); err != nil {
		return err
	}
	var bal int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM ton_balances WHERE account_id=$1 FOR UPDATE`, account).Scan(&bal); err != nil {
		return err
	}
	if bal+delta < 0 {
		return apperr.New(apperr.InsufficientFunds, "not enough TON (balance %s)", tonString(bal))
	}
	if _, err := tx.Exec(ctx, `UPDATE ton_balances SET balance=balance+$2 WHERE account_id=$1`, account, delta); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO ton_ledger (account_id, delta, balance_after, reason, ref) VALUES ($1,$2,$3,$4,$5)`, account, delta, bal+delta, reason, ref)
	return err
}

func (a *app) memo(ctx context.Context, account string) (string, error) {
	var m string
	if err := a.db.QueryRow(ctx, `SELECT memo FROM deposit_memos WHERE account_id=$1`, account).Scan(&m); err == nil {
		return m, nil
	}
	for i := 0; i < 5; i++ {
		raw := make([]byte, 7)
		_, _ = rand.Read(raw)
		m = "OMM" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)[:10]
		tag, err := a.db.Exec(ctx, `INSERT INTO deposit_memos (account_id, memo) VALUES ($1,$2) ON CONFLICT DO NOTHING`, account, m)
		if err == nil && tag.RowsAffected() == 1 {
			return m, nil
		}
		if err == nil {
			if a.db.QueryRow(ctx, `SELECT memo FROM deposit_memos WHERE account_id=$1`, account).Scan(&m) == nil {
				return m, nil
			}
		}
	}
	return "", apperr.New(apperr.Internal, "could not allocate a deposit memo")
}

func (a *app) wallet(ctx context.Context, req c.TokenReq) (c.TonWallet, error) {
	out := c.TonWallet{Ledger: []c.TonLedgerEntry{}, Withdrawals: []c.Withdrawal{}, Policy: a.getPolicy(ctx)}
	_ = a.db.QueryRow(ctx, `SELECT balance, locked FROM ton_balances WHERE account_id=$1`, req.AccountID).Scan(&out.Balance, &out.Locked)
	var err error
	if out.DepositMemo, err = a.memo(ctx, req.AccountID); err != nil {
		return out, err
	}
	if info, err := bus.Request[c.TonInfoResp](ctx, a.bus, c.TonInfo, struct{}{}); err == nil {
		out.DepositAddress, out.Network = info.Address, info.Network
	}
	rows, err := a.db.Query(ctx, `SELECT delta, balance_after, reason, COALESCE(ref,''), created_at FROM ton_ledger WHERE account_id=$1 ORDER BY id DESC LIMIT 30`, req.AccountID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var e c.TonLedgerEntry
		if err := rows.Scan(&e.Delta, &e.Balance, &e.Reason, &e.Ref, &e.CreatedAt); err == nil {
			out.Ledger = append(out.Ledger, e)
		}
	}
	rows.Close()
	out.Withdrawals, err = a.queryWithdrawals(ctx, `WHERE account_id=$1 ORDER BY id DESC LIMIT 10`, req.AccountID)
	return out, err
}

const wdCols = `id, account_id, to_address, amount, fee, status, COALESCE(tx_hash,''), COALESCE(reviewer,''), COALESCE(note,''), created_at, updated_at`

func scanWd(r pgx.Row) (c.Withdrawal, error) {
	var w c.Withdrawal
	err := r.Scan(&w.ID, &w.Account, &w.To, &w.Amount, &w.Fee, &w.Status, &w.TxHash, &w.Reviewer, &w.Note, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, apperr.New(apperr.NotFound, "withdrawal not found")
	}
	return w, err
}

func (a *app) queryWithdrawals(ctx context.Context, where string, args ...any) ([]c.Withdrawal, error) {
	rows, err := a.db.Query(ctx, `SELECT `+wdCols+` FROM ton_withdrawals `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c.Withdrawal{}
	for rows.Next() {
		w, err := scanWd(rows)
		if err != nil {
			return out, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// withdraw moves the amount from balance to locked and queues the request.
// Funds leave the locked bucket only when the ton service reports "sent".
func (a *app) withdraw(ctx context.Context, req c.WithdrawReq) (c.Withdrawal, error) {
	pol := a.getPolicy(ctx)
	if !pol.WithdrawEnabled {
		return c.Withdrawal{}, apperr.New(apperr.Forbidden, "withdrawals are paused")
	}
	if len(req.RequestID) < 8 {
		return c.Withdrawal{}, apperr.New(apperr.Invalid, "request_id required")
	}
	if _, err := address.ParseAddr(req.To); err != nil {
		return c.Withdrawal{}, apperr.New(apperr.Invalid, "invalid TON address")
	}
	if req.Amount < pol.WithdrawMin {
		return c.Withdrawal{}, apperr.New(apperr.Invalid, "minimum withdrawal is %s TON", tonString(pol.WithdrawMin))
	}
	var out c.Withdrawal
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		if w, err := scanWd(tx.QueryRow(ctx, `SELECT `+wdCols+` FROM ton_withdrawals WHERE request_id=$1`, req.RequestID)); err == nil {
			out = w
			return nil
		}
		if err := moveTon(ctx, tx, req.AccountID, -req.Amount, "withdraw_lock", req.To); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE ton_balances SET locked=locked+$2 WHERE account_id=$1`, req.AccountID, req.Amount); err != nil {
			return err
		}
		status := "pending"
		if pol.AutoApproveMax > 0 && req.Amount <= pol.AutoApproveMax {
			status = "approved"
		}
		var err error
		out, err = scanWd(tx.QueryRow(ctx, `INSERT INTO ton_withdrawals (account_id, to_address, amount, fee, status, request_id, reviewer)
			VALUES ($1,$2,$3,$4,$5,$6,CASE WHEN $5='approved' THEN 'auto' END) RETURNING `+wdCols, req.AccountID, req.To, req.Amount, pol.WithdrawFee, status, req.RequestID))
		if err != nil {
			return err
		}
		_, err = appendTx(ctx, tx, "WITHDRAW_REQUEST", 0, req.AccountID, req.To, req.Amount, "TON", map[string]any{"withdrawal_id": out.ID})
		return err
	})
	return out, err
}

// tonCredit is called by the ton service for every incoming transfer.
// It is idempotent on the transaction hash.
func (a *app) tonCredit(ctx context.Context, req c.TonCreditReq) (struct{}, error) {
	if req.TxHash == "" || req.Amount <= 0 {
		return struct{}{}, apperr.New(apperr.Invalid, "bad deposit")
	}
	var account string
	_ = a.db.QueryRow(ctx, `SELECT account_id FROM deposit_memos WHERE memo=$1`, strings.TrimSpace(req.Memo)).Scan(&account)
	credited := false
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		status := "unmatched"
		var acc any
		if account != "" {
			status, acc = "credited", account
		}
		tag, err := tx.Exec(ctx, `INSERT INTO ton_deposits (tx_hash, lt, account_id, amount, memo, sender, status) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			req.TxHash, strconv.FormatUint(req.LT, 10), acc, req.Amount, req.Memo, req.Sender, status)
		if err != nil || tag.RowsAffected() == 0 || account == "" {
			return err
		}
		if err := moveTon(ctx, tx, account, req.Amount, "deposit", req.TxHash); err != nil {
			return err
		}
		credited = true
		_, err = appendTx(ctx, tx, "DEPOSIT", 0, req.Sender, account, req.Amount, "TON", map[string]any{"ton_tx": req.TxHash})
		return err
	})
	if err == nil && credited {
		_ = a.bus.Publish(ctx, c.EvTonDeposit, "deposit:"+req.TxHash, c.TonEv{Account: account, Amount: req.Amount, Ref: req.TxHash})
		a.notifyAccount(ctx, account, map[string]any{"t": "ton_deposit", "amount": req.Amount})
	}
	return struct{}{}, err
}

// notifyAccount pushes to every character of the account (the personal
// channel is per character).
func (a *app) notifyAccount(ctx context.Context, account string, msg map[string]any) {
	chars, err := bus.Request[c.CharacterListResp](ctx, a.bus, c.CharacterList, c.AccountReq{AccountID: account})
	if err != nil {
		return
	}
	var chans []string
	for _, ch := range chars.Characters {
		chans = append(chans, centrifugo.PersonalChannel(ch.ID))
	}
	_ = a.cf.Broadcast(ctx, chans, msg)
}

func (a *app) withdrawals(ctx context.Context, req c.WithdrawalsReq) (c.WithdrawalsResp, error) {
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var list []c.Withdrawal
	var err error
	if req.Status == "" {
		list, err = a.queryWithdrawals(ctx, `ORDER BY id DESC LIMIT $1`, limit)
	} else {
		list, err = a.queryWithdrawals(ctx, `WHERE status=$1 ORDER BY id LIMIT $2`, req.Status, limit)
	}
	return c.WithdrawalsResp{Withdrawals: list}, err
}

// review: admin approves (pending/failed -> approved) or rejects
// (pending/failed -> rejected, funds unlocked back to the balance).
func (a *app) review(ctx context.Context, req c.WithdrawalReview) (c.Withdrawal, error) {
	var out c.Withdrawal
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		w, err := scanWd(tx.QueryRow(ctx, `SELECT `+wdCols+` FROM ton_withdrawals WHERE id=$1 FOR UPDATE`, req.ID))
		if err != nil {
			return err
		}
		if w.Status != "pending" && w.Status != "failed" {
			return apperr.New(apperr.Conflict, "withdrawal is %s", w.Status)
		}
		if req.Approve {
			out, err = scanWd(tx.QueryRow(ctx, `UPDATE ton_withdrawals SET status='approved', reviewer=$2, note=$3, updated_at=now() WHERE id=$1 RETURNING `+wdCols, w.ID, req.Reviewer, req.Note))
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE ton_balances SET locked=locked-$2 WHERE account_id=$1`, w.Account, w.Amount); err != nil {
			return err
		}
		if err := moveTon(ctx, tx, w.Account, w.Amount, "withdraw_rejected", strconv.FormatInt(w.ID, 10)); err != nil {
			return err
		}
		if _, err := appendTx(ctx, tx, "WITHDRAW_REJECT", 0, "", w.Account, w.Amount, "TON", map[string]any{"withdrawal_id": w.ID}); err != nil {
			return err
		}
		out, err = scanWd(tx.QueryRow(ctx, `UPDATE ton_withdrawals SET status='rejected', reviewer=$2, note=$3, updated_at=now() WHERE id=$1 RETURNING `+wdCols, w.ID, req.Reviewer, req.Note))
		return err
	})
	return out, err
}

// result: the ton service reports progress. approved -> sending -> sent|failed.
// A crash between "sending" and the result leaves the row in "sending" for a
// human to check on-chain: it is never re-sent automatically.
func (a *app) result(ctx context.Context, req c.WithdrawalResult) (c.Withdrawal, error) {
	var out c.Withdrawal
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		w, err := scanWd(tx.QueryRow(ctx, `SELECT `+wdCols+` FROM ton_withdrawals WHERE id=$1 FOR UPDATE`, req.ID))
		if err != nil {
			return err
		}
		allowed := map[string]string{"sending": "approved", "sent": "sending", "failed": "sending"}
		if allowed[req.Status] != w.Status {
			return apperr.New(apperr.Conflict, "cannot go from %s to %s", w.Status, req.Status)
		}
		if req.Status == "sent" {
			if _, err := tx.Exec(ctx, `UPDATE ton_balances SET locked=locked-$2 WHERE account_id=$1`, w.Account, w.Amount); err != nil {
				return err
			}
			if _, err := appendTx(ctx, tx, "WITHDRAW", 0, w.Account, w.To, w.Amount-w.Fee, "TON", map[string]any{"withdrawal_id": w.ID, "ton_tx": req.TxHash, "fee": w.Fee}); err != nil {
				return err
			}
		}
		out, err = scanWd(tx.QueryRow(ctx, `UPDATE ton_withdrawals SET status=$2, tx_hash=NULLIF($3,''), note=COALESCE(NULLIF($4,''), note), updated_at=now() WHERE id=$1 RETURNING `+wdCols, w.ID, req.Status, req.TxHash, req.Note))
		return err
	})
	if err == nil && (req.Status == "sent" || req.Status == "failed") {
		_ = a.bus.Publish(ctx, c.EvTonWithdrawn, fmt.Sprintf("withdrawal:%d:%s", out.ID, out.Status), c.TonEv{Account: out.Account, Amount: out.Amount, Ref: out.TxHash, Status: out.Status})
		a.notifyAccount(ctx, out.Account, map[string]any{"t": "ton_withdrawal", "status": out.Status, "amount": out.Amount})
	}
	return out, err
}

func (a *app) stats(ctx context.Context, _ struct{}) (c.AssetStatsResp, error) {
	out := c.AssetStatsResp{Tokens: map[string]int{}}
	rows, err := a.db.Query(ctx, `SELECT state, count(*) FROM tokens GROUP BY state`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var s string
		var n int
		if rows.Scan(&s, &n) == nil {
			out.Tokens[s] = n
		}
	}
	rows.Close()
	_ = a.db.QueryRow(ctx, `SELECT count(*) FILTER (WHERE currency='TON'), count(*) FILTER (WHERE currency='GOLD') FROM listings WHERE status='active'`).Scan(&out.ActiveTon, &out.ActiveGold)
	_ = a.db.QueryRow(ctx, `SELECT COALESCE(sum(price) FILTER (WHERE currency='TON'),0), COALESCE(sum(price) FILTER (WHERE currency='GOLD'),0), count(*)
		FROM listings WHERE status='sold' AND closed_at > now() - interval '24 hours'`).Scan(&out.Volume24hTon, &out.Volume24hGold, &out.Sales24h)
	_ = a.db.QueryRow(ctx, `SELECT COALESCE(balance,0) FROM ton_balances WHERE account_id=$1`, HouseAccount).Scan(&out.FeesTon)
	_ = a.db.QueryRow(ctx, `SELECT COALESCE(sum(balance),0), COALESCE(sum(locked),0) FROM ton_balances WHERE account_id<>$1`, HouseAccount).Scan(&out.UserBalances, &out.LockedTotal)
	_ = a.db.QueryRow(ctx, `SELECT count(*) FROM ton_withdrawals WHERE status IN ('pending','failed','sending')`).Scan(&out.PendingWd)
	_ = a.db.QueryRow(ctx, `SELECT COALESCE(max(height),0) FROM chain_blocks`).Scan(&out.ChainHeight)
	return out, nil
}
