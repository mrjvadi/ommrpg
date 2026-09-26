package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

func fee(price, bps int64) int64 { return price * bps / 10000 }

func (a *app) list(ctx context.Context, req c.ListReq) (c.Listing, error) {
	pol := a.getPolicy(ctx)
	if !pol.TradingEnabled {
		return c.Listing{}, apperr.New(apperr.Forbidden, "trading is paused")
	}
	if len(req.RequestID) < 8 {
		return c.Listing{}, apperr.New(apperr.Invalid, "request_id required")
	}
	switch req.Currency {
	case "TON":
		if req.Price < pol.MinPriceTon {
			return c.Listing{}, apperr.New(apperr.Invalid, "minimum price is %s TON", tonString(pol.MinPriceTon))
		}
	case "GOLD":
		if req.Price < pol.MinPriceGold {
			return c.Listing{}, apperr.New(apperr.Invalid, "minimum price is %d gold", pol.MinPriceGold)
		}
		if err := a.ownsCharacter(ctx, req.AccountID, req.CharacterID); err != nil {
			return c.Listing{}, err
		}
	default:
		return c.Listing{}, apperr.New(apperr.Invalid, "currency must be TON or GOLD")
	}
	var out c.Listing
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var existing int64
		if tx.QueryRow(ctx, `SELECT id FROM listings WHERE request_id=$1`, req.RequestID).Scan(&existing) == nil {
			var err error
			out, err = a.loadListing(ctx, tx, existing)
			return err
		}
		t, err := lockToken(ctx, tx, req.TokenID, req.AccountID)
		if err != nil {
			return err
		}
		if t.State != "vault" || t.ListingID != 0 {
			return apperr.New(apperr.Conflict, "only unlisted tokens in your vault can be sold")
		}
		min := pol.GoldMinRarity
		if req.Currency == "TON" {
			min = pol.TonMinRarity
		}
		if !rarityAtLeast(t.Rarity, min) {
			return apperr.New(apperr.Forbidden, "%s items cannot be sold for %s (needs %s or better)", t.Rarity, req.Currency, min)
		}
		var seller any
		if req.Currency == "GOLD" {
			seller = req.CharacterID
		}
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO listings (token_id, seller_account, seller_character, currency, price, status, request_id)
			VALUES ($1,$2,$3,$4,$5,'active',$6) RETURNING id`, t.ID, req.AccountID, seller, req.Currency, req.Price, req.RequestID).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE tokens SET listing_id=$2, updated_at=now() WHERE id=$1`, t.ID, id); err != nil {
			return err
		}
		if _, err := appendTx(ctx, tx, "LIST", t.ID, req.AccountID, "", req.Price, req.Currency, map[string]any{"listing_id": id}); err != nil {
			return err
		}
		out, err = a.loadListing(ctx, tx, id)
		return err
	})
	if err == nil {
		name, rar := trimName(out.Token.Item)
		_ = a.bus.Publish(ctx, c.EvAssetListed, fmt.Sprintf("listed:%d", out.ID), c.AssetEv{TokenID: out.TokenID, Name: name, Rarity: rar, From: req.AccountID, Currency: out.Currency, Price: out.Price})
	}
	return out, err
}

const listingCols = `l.id, l.token_id, l.seller_account, l.currency, l.price, l.status, COALESCE(l.buyer_account::text,''), l.created_at,
	t.id, t.item_id, t.item, t.item_hash, t.rarity, t.slot, t.owner_account, t.state, COALESCE(t.bound_character::text,''), COALESCE(t.listing_id,0), t.minted_at`

func scanListing(r pgx.Row) (c.Listing, error) {
	var l c.Listing
	var t c.Token
	var item []byte
	err := r.Scan(&l.ID, &l.TokenID, &l.Seller, &l.Currency, &l.Price, &l.Status, &l.Buyer, &l.CreatedAt,
		&t.ID, &t.ItemID, &item, &t.ItemHash, &t.Rarity, &t.Slot, &t.Owner, &t.State, &t.BoundCharacter, &t.ListingID, &t.MintedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, apperr.New(apperr.NotFound, "listing not found")
	}
	t.Item = item
	l.Token = &t
	return l, err
}

func (a *app) loadListing(ctx context.Context, q querier, id int64) (c.Listing, error) {
	return scanListing(q.QueryRow(ctx, `SELECT `+listingCols+` FROM listings l JOIN tokens t ON t.id=l.token_id WHERE l.id=$1`, id))
}

func (a *app) market(ctx context.Context, req c.MarketReq) (c.MarketResp, error) {
	where := []string{}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if req.Status != "" {
		add("l.status=$%d", req.Status)
	} else {
		where = append(where, "l.status='active'")
	}
	if req.Currency != "" {
		add("l.currency=$%d", req.Currency)
	}
	if req.Slot != "" {
		add("t.slot=$%d", req.Slot)
	}
	if req.Rarity != "" {
		add("t.rarity=$%d", req.Rarity)
	}
	if req.Mine {
		add("l.seller_account=$%d", req.AccountID)
	}
	cond := "WHERE " + join(where, " AND ")
	order := "l.id DESC"
	switch req.Sort {
	case "price_asc":
		order = "l.price ASC, l.id"
	case "price_desc":
		order = "l.price DESC, l.id"
	}
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	var total int
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM listings l JOIN tokens t ON t.id=l.token_id `+cond, args...).Scan(&total); err != nil {
		return c.MarketResp{}, err
	}
	rows, err := a.db.Query(ctx, `SELECT `+listingCols+` FROM listings l JOIN tokens t ON t.id=l.token_id `+cond+
		` ORDER BY `+order+` LIMIT `+strconv.Itoa(limit)+` OFFSET `+strconv.Itoa(max(0, req.Offset)), args...)
	if err != nil {
		return c.MarketResp{}, err
	}
	defer rows.Close()
	out := c.MarketResp{Listings: []c.Listing{}, Total: total}
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return out, err
		}
		l.Mine = l.Seller == req.AccountID
		out.Listings = append(out.Listings, l)
	}
	return out, rows.Err()
}

func join(s []string, sep string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += sep
		}
		out += x
	}
	return out
}

func (a *app) closeListing(ctx context.Context, id int64, account string, admin bool) (c.Listing, error) {
	var out c.Listing
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		l, err := scanListing(tx.QueryRow(ctx, `SELECT `+listingCols+` FROM listings l JOIN tokens t ON t.id=l.token_id WHERE l.id=$1 FOR UPDATE OF l`, id))
		if err != nil {
			return err
		}
		if !admin && l.Seller != account {
			return apperr.New(apperr.Forbidden, "not your listing")
		}
		if l.Status != "active" {
			return apperr.New(apperr.Conflict, "listing is %s", l.Status)
		}
		if _, err := tx.Exec(ctx, `UPDATE listings SET status='cancelled', closed_at=now(), updated_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE tokens SET listing_id=NULL, updated_at=now() WHERE id=$1`, l.TokenID); err != nil {
			return err
		}
		if _, err := appendTx(ctx, tx, "DELIST", l.TokenID, l.Seller, "", 0, "", map[string]any{"listing_id": id, "by_admin": admin}); err != nil {
			return err
		}
		l.Status = "cancelled"
		out = l
		return nil
	})
	return out, err
}

func (a *app) cancel(ctx context.Context, req c.BuyReq) (c.Listing, error) {
	return a.closeListing(ctx, req.ListingID, req.AccountID, false)
}

func (a *app) adminCancel(ctx context.Context, req c.BuyReq) (c.Listing, error) {
	return a.closeListing(ctx, req.ListingID, "", true)
}

func (a *app) buy(ctx context.Context, req c.BuyReq) (c.Listing, error) {
	if !a.getPolicy(ctx).TradingEnabled {
		return c.Listing{}, apperr.New(apperr.Forbidden, "trading is paused")
	}
	if len(req.RequestID) < 8 {
		return c.Listing{}, apperr.New(apperr.Invalid, "request_id required")
	}
	l, err := a.loadListing(ctx, a.db, req.ListingID)
	if err != nil {
		return c.Listing{}, err
	}
	if l.Seller == req.AccountID {
		return c.Listing{}, apperr.New(apperr.Conflict, "you cannot buy your own listing")
	}
	if l.Currency == "TON" {
		return a.buyTon(ctx, req)
	}
	return a.buyGold(ctx, req)
}

// buyTon settles entirely inside this database: one transaction moves the
// TON, the fee and the token, and records the SALE on chain.
func (a *app) buyTon(ctx context.Context, req c.BuyReq) (c.Listing, error) {
	var out c.Listing
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		l, err := scanListing(tx.QueryRow(ctx, `SELECT `+listingCols+` FROM listings l JOIN tokens t ON t.id=l.token_id WHERE l.id=$1 FOR UPDATE OF l`, req.ListingID))
		if err != nil {
			return err
		}
		if l.Status == "sold" && l.Buyer == req.AccountID {
			out = l
			return nil // replay
		}
		if l.Status != "active" {
			return apperr.New(apperr.Conflict, "listing is no longer available")
		}
		f := fee(l.Price, a.getPolicy(ctx).FeeBps)
		if err := moveTon(ctx, tx, req.AccountID, -l.Price, "buy", fmt.Sprintf("listing:%d", l.ID)); err != nil {
			return err
		}
		if err := moveTon(ctx, tx, l.Seller, l.Price-f, "sale", fmt.Sprintf("listing:%d", l.ID)); err != nil {
			return err
		}
		if f > 0 {
			if err := moveTon(ctx, tx, HouseAccount, f, "fee", fmt.Sprintf("listing:%d", l.ID)); err != nil {
				return err
			}
		}
		if err := a.completeSale(ctx, tx, l, req.AccountID, f); err != nil {
			return err
		}
		out, err = a.loadListing(ctx, tx, l.ID)
		return err
	})
	if err == nil {
		a.announceSale(ctx, out)
	}
	return out, err
}

func (a *app) completeSale(ctx context.Context, tx pgx.Tx, l c.Listing, buyer string, f int64) error {
	if _, err := tx.Exec(ctx, `UPDATE listings SET status='sold', buyer_account=$2, fee=$3, closed_at=now(), updated_at=now() WHERE id=$1`, l.ID, buyer, f); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE tokens SET owner_account=$2, listing_id=NULL, updated_at=now() WHERE id=$1`, l.TokenID, buyer); err != nil {
		return err
	}
	_, err := appendTx(ctx, tx, "SALE", l.TokenID, l.Seller, buyer, l.Price, l.Currency, map[string]any{"listing_id": l.ID, "fee": f})
	return err
}

func (a *app) announceSale(ctx context.Context, l c.Listing) {
	name, rar := trimName(l.Token.Item)
	_ = a.bus.Publish(ctx, c.EvAssetSold, fmt.Sprintf("sold:%d", l.ID), c.AssetEv{TokenID: l.TokenID, Name: name, Rarity: rar, From: l.Seller, To: l.Buyer, Currency: l.Currency, Price: l.Price})
}

// buyGold is a saga: reserve the listing, pay through item-service (gold
// lives in character wallets there), then hand over the token.
func (a *app) buyGold(ctx context.Context, req c.BuyReq) (c.Listing, error) {
	if err := a.ownsCharacter(ctx, req.AccountID, req.CharacterID); err != nil {
		return c.Listing{}, err
	}
	op := fmt.Sprintf("buy:%d:%s", req.ListingID, req.RequestID)
	var l c.Listing
	var sellerChar string
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var err error
		l, err = scanListing(tx.QueryRow(ctx, `SELECT `+listingCols+` FROM listings l JOIN tokens t ON t.id=l.token_id WHERE l.id=$1 FOR UPDATE OF l`, req.ListingID))
		if err != nil {
			return err
		}
		if l.Status != "active" {
			return apperr.New(apperr.Conflict, "listing is no longer available")
		}
		if err := tx.QueryRow(ctx, `SELECT seller_character::text FROM listings WHERE id=$1`, l.ID).Scan(&sellerChar); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE listings SET status='settling', buyer_account=$2, buyer_character=$3, op_id=$4, updated_at=now() WHERE id=$1`,
			l.ID, req.AccountID, req.CharacterID, op)
		return err
	})
	if err != nil {
		return c.Listing{}, err
	}
	return a.settleGold(ctx, l.ID, op, req.CharacterID, sellerChar, req.AccountID, l.Price)
}

func (a *app) settleGold(ctx context.Context, id int64, op, buyerChar, sellerChar, buyer string, price int64) (c.Listing, error) {
	f := fee(price, a.getPolicy(ctx).FeeBps)
	_, err := bus.Request[c.GoldTransferResp](ctx, a.bus, c.ItemGoldTransfer, c.GoldTransferReq{
		From: buyerChar, To: sellerChar, Amount: price, Fee: f, OpID: op, Reason: "market",
	})
	if err != nil {
		if definitive(err) {
			_, _ = a.db.Exec(ctx, `UPDATE listings SET status='active', buyer_account=NULL, buyer_character=NULL, op_id=NULL, updated_at=now() WHERE id=$1 AND op_id=$2`, id, op)
		}
		return c.Listing{}, err
	}
	var out c.Listing
	err = pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		l, err := scanListing(tx.QueryRow(ctx, `SELECT `+listingCols+` FROM listings l JOIN tokens t ON t.id=l.token_id WHERE l.id=$1 FOR UPDATE OF l`, id))
		if err != nil {
			return err
		}
		if l.Status == "sold" {
			out = l
			return nil
		}
		if err := a.completeSale(ctx, tx, l, buyer, f); err != nil {
			return err
		}
		out, err = a.loadListing(ctx, tx, id)
		return err
	})
	if err == nil {
		a.announceSale(ctx, out)
	}
	return out, err
}

func (a *app) reconcileListings(ctx context.Context) {
	rows, err := a.db.Query(ctx, `SELECT id, op_id, buyer_character::text, seller_character::text, buyer_account::text, price FROM listings
		WHERE status='settling' AND updated_at < now() - interval '10 seconds' LIMIT 50`)
	if err != nil {
		return
	}
	type job struct {
		id                int64
		op, bc, sc, buyer string
		price             int64
	}
	var jobs []job
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.op, &j.bc, &j.sc, &j.buyer, &j.price) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	for _, j := range jobs {
		if _, err := a.settleGold(ctx, j.id, j.op, j.bc, j.sc, j.buyer, j.price); err != nil {
			a.log.Warn("reconcile listing", "id", j.id, "err", err)
		}
	}
}
