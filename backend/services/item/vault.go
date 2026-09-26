package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/items"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// These handlers are the item side of asset-service sagas. Every one is
// idempotent on its op id, so the orchestrator can retry after timeouts.

type vaultRow struct {
	*stored
	State   string
	VaultOp string
	ClaimOp string
}

func (a *app) lockAny(ctx context.Context, tx pgx.Tx, id string) (*vaultRow, error) {
	var st, vop, cop string
	row := tx.QueryRow(ctx, `SELECT `+itemCols+`, state, COALESCE(vault_op,''), COALESCE(claim_op,'') FROM items WHERE id=$1 FOR UPDATE`, id)
	s := &stored{}
	var raw []byte
	err := row.Scan(&s.ID, &s.Owner, &raw, &s.State.Enhance, &s.State.Level, &s.State.XP, &s.Attempts, &s.Equipped, &s.Source, &s.CreatedAt, &st, &vop, &cop)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.New(apperr.NotFound, "item not found")
	}
	if err != nil {
		return nil, err
	}
	if err := jsonUnmarshal(raw, &s.Item); err != nil {
		return nil, err
	}
	return &vaultRow{stored: s, State: st, VaultOp: vop, ClaimOp: cop}, nil
}

func (a *app) vault(ctx context.Context, req c.VaultReq) (c.OwnedItem, error) {
	var out c.OwnedItem
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		it, err := a.lockAny(ctx, tx, req.ItemID)
		if err != nil {
			return err
		}
		if it.State == "vault" && it.VaultOp == req.OpID {
			out = view(it.stored)
			return nil
		}
		if it.State != "inventory" || it.Owner != req.CharacterID {
			return apperr.New(apperr.Forbidden, "item is not in your bag")
		}
		if it.Equipped != "" {
			return apperr.New(apperr.Conflict, "unequip the item first")
		}
		if _, err := tx.Exec(ctx, `UPDATE items SET state='vault', vault_op=$2 WHERE id=$1`, it.ID, req.OpID); err != nil {
			return err
		}
		out = view(it.stored)
		return nil
	})
	return out, err
}

func (a *app) unvault(ctx context.Context, req c.VaultReq) (struct{}, error) {
	_, err := a.db.Exec(ctx, `UPDATE items SET state='inventory', vault_op=NULL WHERE id=$1 AND state='vault' AND vault_op=$2`, req.ItemID, req.OpID)
	return struct{}{}, err
}

func (a *app) claim(ctx context.Context, req c.VaultReq) (c.OwnedItem, error) {
	var out c.OwnedItem
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		it, err := a.lockAny(ctx, tx, req.ItemID)
		if err != nil {
			return err
		}
		if it.State == "inventory" && it.ClaimOp == req.OpID && it.Owner == req.CharacterID {
			out = view(it.stored)
			return nil
		}
		if it.State != "vault" {
			return apperr.New(apperr.Conflict, "item is not in the vault")
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM items WHERE owner_id=$1 AND state='inventory'`, req.CharacterID).Scan(&n); err != nil {
			return err
		}
		if n >= a.maxInv {
			return apperr.New(apperr.Conflict, "your bag is full")
		}
		if _, err := tx.Exec(ctx, `UPDATE items SET state='inventory', owner_id=$2, claim_op=$3, equipped_slot=NULL WHERE id=$1`, it.ID, req.CharacterID, req.OpID); err != nil {
			return err
		}
		it.Owner = req.CharacterID
		out = view(it.stored)
		return nil
	})
	return out, err
}

func (a *app) snapshot(ctx context.Context, req c.VaultReq) (c.OwnedItem, error) {
	var out c.OwnedItem
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		it, err := a.lockAny(ctx, tx, req.ItemID)
		if err == nil {
			out = view(it.stored)
		}
		return err
	})
	return out, err
}

// goldTransfer pays gold from one character to another (marketplace GOLD
// sales). The fee is burned: a gold sink.
func (a *app) goldTransfer(ctx context.Context, req c.GoldTransferReq) (c.GoldTransferResp, error) {
	if req.Amount <= 0 || req.Fee < 0 || req.Fee > req.Amount || req.From == req.To {
		return c.GoldTransferResp{}, apperr.New(apperr.Invalid, "bad transfer")
	}
	var out c.GoldTransferResp
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		// lock both wallets in a fixed order to avoid deadlocks
		first, second := req.From, req.To
		if second < first {
			first, second = second, first
		}
		for _, k := range []string{first, second} {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "wallet:"+k); err != nil {
				return err
			}
		}
		prev, ok, err := idempotent[c.GoldTransferResp](ctx, tx, req.OpID, req.From)
		if err != nil || ok {
			out = prev
			return err
		}
		w, err := wallet(ctx, tx, req.From, true)
		if err != nil {
			return err
		}
		if w.Gold < req.Amount {
			return apperr.New(apperr.InsufficientFunds, "not enough gold (need %d)", req.Amount)
		}
		if out.FromWallet, err = credit(ctx, tx, req.From, -req.Amount, 0, req.Reason, req.OpID); err != nil {
			return err
		}
		if _, err := credit(ctx, tx, req.To, req.Amount-req.Fee, 0, req.Reason, req.OpID); err != nil {
			return err
		}
		return remember(ctx, tx, req.OpID, req.From, out)
	})
	return out, err
}

// adminGrant is used by the admin panel (always audited there).
func (a *app) adminGrant(ctx context.Context, req c.AdminGrantReq) (c.InventoryResp, error) {
	if req.OpID == "" {
		return c.InventoryResp{}, apperr.New(apperr.Invalid, "op_id required")
	}
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		if _, ok, err := idempotent[struct{}](ctx, tx, req.OpID, req.CharacterID); err != nil || ok {
			return err
		}
		if req.Gold != 0 || req.Essence != 0 {
			w, err := wallet(ctx, tx, req.CharacterID, true)
			if err != nil {
				return err
			}
			if w.Gold+req.Gold < 0 || w.Essence+req.Essence < 0 {
				return apperr.New(apperr.InsufficientFunds, "balance would become negative")
			}
			if _, err := credit(ctx, tx, req.CharacterID, req.Gold, req.Essence, "admin:"+req.Reason, req.OpID); err != nil {
				return err
			}
		}
		if req.Base != "" {
			var rar items.Rarity
			if err := rar.UnmarshalText([]byte(req.Rarity)); err != nil {
				return apperr.New(apperr.Invalid, "bad rarity")
			}
			it, ok := items.GenerateBase(seed.FromString(req.OpID), req.Base, max(1, req.ItemLevel), rar)
			if !ok {
				return apperr.New(apperr.Invalid, "unknown base %q", req.Base)
			}
			g, err := a.grant(ctx, tx, req.CharacterID, it, "admin", "admin:"+req.OpID, false)
			if err != nil {
				return err
			}
			if g != nil && req.Enhance > 0 {
				e := min(req.Enhance, rar.MaxEnhance())
				if _, err := tx.Exec(ctx, `UPDATE items SET enhance=$2 WHERE id=$1`, g.ID, e); err != nil {
					return err
				}
			}
		}
		return remember(ctx, tx, req.OpID, req.CharacterID, struct{}{})
	})
	if err != nil {
		return c.InventoryResp{}, err
	}
	return a.list(ctx, c.ItemActionReq{CharacterID: req.CharacterID})
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
