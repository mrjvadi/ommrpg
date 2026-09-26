package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/chain"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/items"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
)

// HouseAccount receives marketplace fees.
const HouseAccount = "00000000-0000-0000-0000-000000000000"

type app struct {
	db     *pgxpool.Pool
	bus    *bus.Bus
	cf     *centrifugo.Client
	log    *slog.Logger
	signer *chain.Signer
	policy *lru.Cache[string, c.MarketPolicy]
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------- chain txs

// appendTx records a movement on OMM Chain inside the caller's transaction,
// so the chain and the business state can never disagree.
func appendTx(ctx context.Context, tx pgx.Tx, kind string, tokenID int64, from, to string, amount int64, currency string, payload any) (string, error) {
	var raw json.RawMessage
	if payload != nil {
		raw, _ = json.Marshal(payload)
	}
	t := chain.Tx{Kind: kind, TokenID: tokenID, From: from, To: to, Amount: amount, Currency: currency, Payload: raw, Time: time.Now().UnixNano(), Nonce: randToken(8)}
	h := t.Hash()
	var tok any
	if tokenID > 0 {
		tok = tokenID
	}
	_, err := tx.Exec(ctx, `INSERT INTO chain_txs (hash, kind, token_id, from_party, to_party, amount, currency, payload, created_at)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NULLIF($7,''),$8,$9)`,
		h, kind, tok, from, to, amount, currency, []byte(raw), time.Unix(0, t.Time))
	return h, err
}

// ---------------------------------------------------------------- policy

var defaultPolicy = c.MarketPolicy{
	MintMinRarity: "rare", GoldMinRarity: "rare", TonMinRarity: "epic",
	FeeBps: 250, MinPriceTon: 100_000_000, MinPriceGold: 10,
	WithdrawMin: 500_000_000, WithdrawFee: 50_000_000, AutoApproveMax: 0,
	TradingEnabled: true, WithdrawEnabled: true,
}

func (a *app) getPolicy(ctx context.Context) c.MarketPolicy {
	if p, ok := a.policy.Get("p"); ok {
		return p
	}
	p := defaultPolicy
	var raw []byte
	if err := a.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='policy'`).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &p)
	}
	a.policy.Put("p", p)
	return p
}

func (a *app) policyGet(ctx context.Context, _ struct{}) (c.MarketPolicy, error) {
	return a.getPolicy(ctx), nil
}

func validRarity(r string) bool {
	var x items.Rarity
	return x.UnmarshalText([]byte(r)) == nil
}

func (a *app) policySet(ctx context.Context, p c.MarketPolicy) (c.MarketPolicy, error) {
	if !validRarity(p.MintMinRarity) || !validRarity(p.GoldMinRarity) || !validRarity(p.TonMinRarity) {
		return p, apperr.New(apperr.Invalid, "unknown rarity")
	}
	if p.FeeBps < 0 || p.FeeBps > 2000 || p.MinPriceTon <= 0 || p.MinPriceGold <= 0 || p.WithdrawFee < 0 || p.WithdrawMin <= p.WithdrawFee {
		return p, apperr.New(apperr.Invalid, "policy values out of range")
	}
	raw, _ := json.Marshal(p)
	if _, err := a.db.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('policy', $1) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, raw); err != nil {
		return p, err
	}
	a.policy.Delete("p")
	return p, nil
}

func rarityAtLeast(r, min string) bool {
	var a, b items.Rarity
	if a.UnmarshalText([]byte(r)) != nil || b.UnmarshalText([]byte(min)) != nil {
		return false
	}
	return a >= b
}

// ---------------------------------------------------------------- tokens

const tokenCols = `id, item_id, item, item_hash, rarity, slot, owner_account, state, COALESCE(bound_character::text,''), COALESCE(listing_id,0), minted_at`

func scanToken(r pgx.Row) (c.Token, error) {
	var t c.Token
	var item []byte
	err := r.Scan(&t.ID, &t.ItemID, &item, &t.ItemHash, &t.Rarity, &t.Slot, &t.Owner, &t.State, &t.BoundCharacter, &t.ListingID, &t.MintedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, apperr.New(apperr.NotFound, "token not found")
	}
	t.Item = item
	return t, err
}

func itemHash(raw []byte) string {
	s := sha256.Sum256(raw)
	return hex.EncodeToString(s[:])
}

// ownsCharacter checks that a character belongs to the account.
func (a *app) ownsCharacter(ctx context.Context, account, character string) error {
	if character == "" {
		return apperr.New(apperr.Invalid, "select a character first")
	}
	_, err := bus.Request[c.Character](ctx, a.bus, c.CharacterGet, c.CharacterReq{CharacterID: character, AccountID: account})
	return err
}

// definitive reports whether a remote error means "this will never
// succeed" (so the saga should roll back) rather than "try again".
func definitive(err error) bool {
	e := apperr.From(err)
	return e.Code != apperr.Unavailable && e.Code != apperr.Internal
}

func (a *app) mint(ctx context.Context, req c.TokenReq) (c.Token, error) {
	if len(req.RequestID) < 8 {
		return c.Token{}, apperr.New(apperr.Invalid, "request_id required")
	}
	if err := a.ownsCharacter(ctx, req.AccountID, req.CharacterID); err != nil {
		return c.Token{}, err
	}
	op := "mint:" + req.AccountID + ":" + req.RequestID
	if t, err := scanToken(a.db.QueryRow(ctx, `SELECT `+tokenCols+` FROM tokens WHERE op_id=$1 OR (item_id=$2 AND state<>'pending')`, op, req.ItemID)); err == nil {
		if t.Owner == req.AccountID && t.State != "pending" {
			return t, nil // replay
		}
		return c.Token{}, apperr.New(apperr.Conflict, "this item is already an NFT")
	}
	snap, err := bus.Request[c.OwnedItem](ctx, a.bus, c.ItemSnapshot, c.VaultReq{ItemID: req.ItemID})
	if err != nil {
		return c.Token{}, err
	}
	pol := a.getPolicy(ctx)
	if !rarityAtLeast(snap.Item.Rarity.String(), pol.MintMinRarity) {
		return c.Token{}, apperr.New(apperr.Forbidden, "only %s or better items can become NFTs", pol.MintMinRarity)
	}
	var id int64
	err = a.db.QueryRow(ctx, `INSERT INTO tokens (item_id, owner_account, state, op_id, op_character, rarity, slot)
		VALUES ($1,$2,'pending',$3,$4,$5,$6) RETURNING id`, req.ItemID, req.AccountID, op, req.CharacterID, snap.Item.Rarity.String(), string(snap.Item.Slot)).Scan(&id)
	if err != nil {
		return c.Token{}, apperr.New(apperr.Conflict, "this item is already being minted")
	}
	return a.finishMint(ctx, id, op, req.CharacterID, req.ItemID)
}

// finishMint runs (or re-runs) the item side of a mint and seals it.
func (a *app) finishMint(ctx context.Context, id int64, op, character, itemID string) (c.Token, error) {
	snap, err := bus.Request[c.OwnedItem](ctx, a.bus, c.ItemVault, c.VaultReq{CharacterID: character, ItemID: itemID, OpID: op})
	if err != nil {
		if definitive(err) {
			_, _ = a.db.Exec(ctx, `DELETE FROM tokens WHERE id=$1 AND state='pending'`, id)
		}
		return c.Token{}, err
	}
	raw, _ := json.Marshal(snap)
	var t c.Token
	err = pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var owner string
		if err := tx.QueryRow(ctx, `UPDATE tokens SET state='vault', item=$2, item_hash=$3, op_id=NULL, op_character=NULL, updated_at=now()
			WHERE id=$1 AND state='pending' RETURNING owner_account`, id, raw, itemHash(raw)).Scan(&owner); err != nil {
			return err
		}
		if _, err := appendTx(ctx, tx, "MINT", id, "", owner, 0, "", map[string]any{"item_id": itemID, "item_hash": itemHash(raw), "name": snap.Item.Name, "rarity": snap.Item.Rarity}); err != nil {
			return err
		}
		t, err = scanToken(tx.QueryRow(ctx, `SELECT `+tokenCols+` FROM tokens WHERE id=$1`, id))
		return err
	})
	if err == nil {
		_ = a.bus.Publish(ctx, c.EvAssetMinted, "minted:"+itemID, c.AssetEv{TokenID: id, Name: snap.Item.Name, Rarity: snap.Item.Rarity.String(), To: t.Owner})
	}
	return t, err
}

// lockToken loads a token for update and checks ownership.
func lockToken(ctx context.Context, tx pgx.Tx, id int64, account string) (c.Token, error) {
	t, err := scanToken(tx.QueryRow(ctx, `SELECT `+tokenCols+` FROM tokens WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return t, err
	}
	if account != "" && t.Owner != account {
		return t, apperr.New(apperr.Forbidden, "not your token")
	}
	return t, nil
}

// claim moves a vaulted token's item onto one of the owner's characters.
func (a *app) claim(ctx context.Context, req c.TokenReq) (c.Token, error) {
	if err := a.ownsCharacter(ctx, req.AccountID, req.CharacterID); err != nil {
		return c.Token{}, err
	}
	op := "claim:" + randToken(8)
	var t c.Token
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var err error
		if t, err = lockToken(ctx, tx, req.TokenID, req.AccountID); err != nil {
			return err
		}
		if t.State != "vault" || t.ListingID != 0 {
			return apperr.New(apperr.Conflict, "the token must be in your vault and not listed")
		}
		_, err = tx.Exec(ctx, `UPDATE tokens SET state='claiming', op_id=$2, op_character=$3, op_prev_state='vault', updated_at=now() WHERE id=$1`, t.ID, op, req.CharacterID)
		return err
	})
	if err != nil {
		return c.Token{}, err
	}
	return a.finishClaim(ctx, t.ID, op, req.CharacterID, t.ItemID)
}

func (a *app) finishClaim(ctx context.Context, id int64, op, character, itemID string) (c.Token, error) {
	snap, err := bus.Request[c.OwnedItem](ctx, a.bus, c.ItemClaim, c.VaultReq{CharacterID: character, ItemID: itemID, OpID: op})
	if err != nil {
		if definitive(err) {
			_, _ = a.db.Exec(ctx, `UPDATE tokens SET state='vault', op_id=NULL, op_character=NULL WHERE id=$1 AND op_id=$2`, id, op)
		}
		return c.Token{}, err
	}
	raw, _ := json.Marshal(snap)
	var t c.Token
	err = pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var owner string
		if err := tx.QueryRow(ctx, `UPDATE tokens SET state='bound', bound_character=$3, item=$4, item_hash=$5, op_id=NULL, op_character=NULL, updated_at=now()
			WHERE id=$1 AND op_id=$2 RETURNING owner_account`, id, op, character, raw, itemHash(raw)).Scan(&owner); err != nil {
			return err
		}
		if _, err := appendTx(ctx, tx, "BIND", id, owner, character, 0, "", nil); err != nil {
			return err
		}
		t, err = scanToken(tx.QueryRow(ctx, `SELECT `+tokenCols+` FROM tokens WHERE id=$1`, id))
		return err
	})
	return t, err
}

// depositToken puts a bound token's item back into the vault (to trade it).
func (a *app) depositToken(ctx context.Context, req c.TokenReq) (c.Token, error) {
	op := "deposit:" + randToken(8)
	var t c.Token
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var err error
		if t, err = lockToken(ctx, tx, req.TokenID, req.AccountID); err != nil {
			return err
		}
		if t.State != "bound" {
			return apperr.New(apperr.Conflict, "the token is not on a character")
		}
		_, err = tx.Exec(ctx, `UPDATE tokens SET state='depositing', op_id=$2, op_character=bound_character, op_prev_state='bound', updated_at=now() WHERE id=$1`, t.ID, op)
		return err
	})
	if err != nil {
		return c.Token{}, err
	}
	return a.finishDeposit(ctx, t.ID, op, t.BoundCharacter, t.ItemID)
}

func (a *app) finishDeposit(ctx context.Context, id int64, op, character, itemID string) (c.Token, error) {
	snap, err := bus.Request[c.OwnedItem](ctx, a.bus, c.ItemVault, c.VaultReq{CharacterID: character, ItemID: itemID, OpID: op})
	if err != nil {
		if definitive(err) {
			_, _ = a.db.Exec(ctx, `UPDATE tokens SET state='bound', op_id=NULL, op_character=NULL WHERE id=$1 AND op_id=$2`, id, op)
		}
		return c.Token{}, err
	}
	raw, _ := json.Marshal(snap)
	var t c.Token
	err = pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var owner string
		if err := tx.QueryRow(ctx, `UPDATE tokens SET state='vault', bound_character=NULL, item=$3, item_hash=$4, op_id=NULL, op_character=NULL, updated_at=now()
			WHERE id=$1 AND op_id=$2 RETURNING owner_account`, id, op, raw, itemHash(raw)).Scan(&owner); err != nil {
			return err
		}
		if _, err := appendTx(ctx, tx, "VAULT", id, character, owner, 0, "", map[string]any{"item_hash": itemHash(raw)}); err != nil {
			return err
		}
		t, err = scanToken(tx.QueryRow(ctx, `SELECT `+tokenCols+` FROM tokens WHERE id=$1`, id))
		return err
	})
	return t, err
}

// burn destroys the token; the item stays on its character as a normal item.
func (a *app) burn(ctx context.Context, req c.TokenReq) (c.Token, error) {
	var t c.Token
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var err error
		if t, err = lockToken(ctx, tx, req.TokenID, req.AccountID); err != nil {
			return err
		}
		if t.State != "bound" {
			return apperr.New(apperr.Conflict, "claim the token onto a character before burning it")
		}
		if _, err := tx.Exec(ctx, `UPDATE tokens SET state='burned', updated_at=now() WHERE id=$1`, t.ID); err != nil {
			return err
		}
		_, err = appendTx(ctx, tx, "BURN", t.ID, t.Owner, "", 0, "", nil)
		t.State = "burned"
		return err
	})
	return t, err
}

func (a *app) tokens(ctx context.Context, req c.TokenReq) (c.TokensResp, error) {
	rows, err := a.db.Query(ctx, `SELECT `+tokenCols+` FROM tokens WHERE owner_account=$1 AND state NOT IN ('burned','pending') ORDER BY id DESC`, req.AccountID)
	if err != nil {
		return c.TokensResp{}, err
	}
	defer rows.Close()
	out := c.TokensResp{Tokens: []c.Token{}}
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return out, err
		}
		out.Tokens = append(out.Tokens, t)
	}
	return out, rows.Err()
}

func (a *app) token(ctx context.Context, req c.TokenReq) (c.TokenView, error) {
	t, err := scanToken(a.db.QueryRow(ctx, `SELECT `+tokenCols+` FROM tokens WHERE id=$1`, req.TokenID))
	if err != nil {
		return c.TokenView{}, err
	}
	txs, err := a.txs(ctx, `WHERE token_id=$1 ORDER BY id`, t.ID)
	return c.TokenView{Token: t, Provenance: txs}, err
}

func (a *app) txs(ctx context.Context, where string, args ...any) ([]c.ChainTx, error) {
	rows, err := a.db.Query(ctx, `SELECT id, hash, kind, COALESCE(token_id,0), COALESCE(from_party,''), COALESCE(to_party,''), COALESCE(amount,0), COALESCE(currency,''), payload, block_height, created_at FROM chain_txs `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c.ChainTx{}
	for rows.Next() {
		var t c.ChainTx
		var payload []byte
		if err := rows.Scan(&t.ID, &t.Hash, &t.Kind, &t.TokenID, &t.From, &t.To, &t.Amount, &t.Currency, &payload, &t.BlockHeight, &t.CreatedAt); err != nil {
			return out, err
		}
		if len(payload) > 0 {
			t.Payload = payload
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- reconciler

// reconciler finishes sagas interrupted by timeouts or crashes. Every step
// it repeats is idempotent on the saga's op id.
func (a *app) reconciler(ctx context.Context) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		rows, err := a.db.Query(ctx, `SELECT id, state, op_id, COALESCE(op_character::text,''), item_id FROM tokens
			WHERE op_id IS NOT NULL AND updated_at < now() - interval '10 seconds' LIMIT 50`)
		if err != nil {
			continue
		}
		type job struct {
			id                    int64
			state, op, char, item string
		}
		var jobs []job
		for rows.Next() {
			var j job
			if rows.Scan(&j.id, &j.state, &j.op, &j.char, &j.item) == nil {
				jobs = append(jobs, j)
			}
		}
		rows.Close()
		for _, j := range jobs {
			var err error
			switch j.state {
			case "pending":
				_, err = a.finishMint(ctx, j.id, j.op, j.char, j.item)
			case "claiming":
				_, err = a.finishClaim(ctx, j.id, j.op, j.char, j.item)
			case "depositing":
				_, err = a.finishDeposit(ctx, j.id, j.op, j.char, j.item)
			}
			if err != nil {
				a.log.Warn("reconcile token", "id", j.id, "state", j.state, "err", err)
			}
		}
		a.reconcileListings(ctx)
	}
}

func trimName(raw json.RawMessage) (name, rarity string) {
	var v struct {
		Name string `json:"display_name"`
		Item struct {
			Rarity string `json:"rarity"`
		} `json:"item"`
	}
	_ = json.Unmarshal(raw, &v)
	return strings.TrimSpace(v.Name), v.Item.Rarity
}
