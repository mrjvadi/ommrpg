package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

// runMarket exercises NFTs, the TON and gold marketplace, withdrawals and
// OMM Chain proofs end to end. It needs direct NATS access (-nats) to play
// the admin (grant a test item, approve a withdrawal) and the TON network
// (mock deposits).
func runMarket(natsURL string) error {
	lg := log.New(os.Stdout, "[market] ", log.Ltime)
	nb, err := bus.Connect(natsURL, "bot", nil_logger())
	if err != nil {
		return err
	}
	defer nb.Close()
	ctx := context.Background()
	seller := &bot{grid: map[[2]int]bool{}, name: "sell" + randHex(4), log: lg}
	buyer := &bot{grid: map[[2]int]bool{}, name: "buy" + randHex(4), log: lg}
	for _, b := range []*bot{seller, buyer} {
		if err := b.loginAndSelect(); err != nil {
			return err
		}
	}
	// admin: grant the seller an epic fire-capable sword at +10
	var inv c.InventoryResp
	inv, err = bus.Request[c.InventoryResp](ctx, nb, c.ItemAdminGrant, c.AdminGrantReq{
		CharacterID: seller.char.ID, Base: "sword", Rarity: "epic", ItemLevel: 12, Enhance: 10, Reason: "e2e", OpID: "e2e:" + randHex(8)})
	if err != nil {
		return fmt.Errorf("grant: %w", err)
	}
	var item c.OwnedItem
	for _, it := range inv.Items {
		if it.Source == "admin" {
			item = it
		}
	}
	lg.Printf("granted %q (%s, +%d)", item.DisplayName, item.Item.Rarity, item.State.Enhance)

	// mint -> vault
	var tok c.Token
	if err := seller.http("POST", "/api/v1/vault/mint", map[string]string{"item_id": item.ID, "request_id": randHex(8)}, &tok); err != nil {
		return err
	}
	if tok.State != "vault" {
		return fmt.Errorf("token state %s", tok.State)
	}
	lg.Printf("minted token #%d", tok.ID)
	var after c.InventoryResp
	_ = seller.http("GET", "/api/v1/inventory", nil, &after)
	for _, it := range after.Items {
		if it.ID == item.ID {
			return errors.New("vaulted item still in the bag")
		}
	}

	// list for 1.5 TON
	price := int64(1_500_000_000)
	var lst c.Listing
	if err := seller.http("POST", fmt.Sprintf("/api/v1/vault/%d/list", tok.ID), map[string]any{"currency": "TON", "price": price, "request_id": randHex(8)}, &lst); err != nil {
		return err
	}
	lg.Printf("listed #%d for %d nanoTON", lst.ID, lst.Price)

	// buyer tops up 3 TON through a (mock) on-chain deposit with their memo
	var wal c.TonWallet
	if err := buyer.http("GET", "/api/v1/ton", nil, &wal); err != nil {
		return err
	}
	if _, err := bus.Request[struct{}](ctx, nb, c.TonMockCredit, c.MockDepositReq{Memo: wal.DepositMemo, Amount: 3_000_000_000}); err != nil {
		return err
	}
	if err := buyer.waitFor(func() bool { _ = buyer.http("GET", "/api/v1/ton", nil, &wal); return wal.Balance == 3_000_000_000 }, 30*time.Second); err != nil {
		return errors.New("deposit never credited")
	}
	lg.Printf("buyer deposit credited (memo %s)", wal.DepositMemo)

	// buyer finds and buys it
	var mk c.MarketResp
	if err := buyer.http("GET", "/api/v1/market?currency=TON", nil, &mk); err != nil {
		return err
	}
	found := false
	for _, l := range mk.Listings {
		found = found || l.ID == lst.ID
	}
	if !found {
		return errors.New("listing not in market")
	}
	var sold c.Listing
	if err := buyer.http("POST", fmt.Sprintf("/api/v1/market/%d/buy", lst.ID), map[string]string{"request_id": randHex(8)}, &sold); err != nil {
		return err
	}
	if sold.Status != "sold" || sold.Token.Owner != buyer.account {
		return fmt.Errorf("sale not settled: %+v", sold)
	}
	var sw c.TonWallet
	_ = seller.http("GET", "/api/v1/ton", nil, &sw)
	wantSeller := price - price*sw.Policy.FeeBps/10000
	if sw.Balance != wantSeller {
		return fmt.Errorf("seller balance %d, want %d", sw.Balance, wantSeller)
	}
	lg.Printf("sold; seller received %d nanoTON after fee", sw.Balance)
	// a second buy of the same listing must fail
	if err := seller.http("POST", fmt.Sprintf("/api/v1/market/%d/buy", lst.ID), map[string]string{"request_id": randHex(8)}, nil); err == nil {
		return errors.New("double sale allowed")
	}

	// buyer claims it onto their character and equips it
	var claimed c.Token
	if err := buyer.http("POST", fmt.Sprintf("/api/v1/vault/%d/claim", tok.ID), map[string]string{}, &claimed); err != nil {
		return err
	}
	if claimed.State != "bound" {
		return fmt.Errorf("claim state %s", claimed.State)
	}
	var binv c.InventoryResp
	if err := buyer.http("POST", "/api/v1/inventory/"+item.ID+"/equip", map[string]string{}, &binv); err != nil {
		return err
	}
	var me c.Character
	_ = buyer.http("GET", "/api/v1/character", nil, &me)
	if me.FX == nil || me.FX.Tier < 2 {
		return fmt.Errorf("+10 weapon should glow, fx=%+v", me.FX)
	}
	lg.Printf("buyer equipped it: fx %+v", *me.FX)

	// resell for gold: back to the vault, list for gold, the seller buys it back
	if err := buyer.http("POST", "/api/v1/inventory/unequip", map[string]string{"slot": "weapon"}, nil); err != nil {
		return err
	}
	if err := buyer.http("POST", fmt.Sprintf("/api/v1/vault/%d/deposit", tok.ID), map[string]string{}, nil); err != nil {
		return err
	}
	var gl c.Listing
	if err := buyer.http("POST", fmt.Sprintf("/api/v1/vault/%d/list", tok.ID), map[string]any{"currency": "GOLD", "price": 60, "request_id": randHex(8)}, &gl); err != nil {
		return err
	}
	var before c.InventoryResp
	_ = seller.http("GET", "/api/v1/inventory", nil, &before)
	var gs c.Listing
	if err := seller.http("POST", fmt.Sprintf("/api/v1/market/%d/buy", gl.ID), map[string]string{"request_id": randHex(8)}, &gs); err != nil {
		return err
	}
	var afterGold c.InventoryResp
	_ = seller.http("GET", "/api/v1/inventory", nil, &afterGold)
	if gs.Status != "sold" || before.Wallet.Gold-afterGold.Wallet.Gold != 60 {
		return fmt.Errorf("gold sale wrong: %s, gold %d -> %d", gs.Status, before.Wallet.Gold, afterGold.Wallet.Gold)
	}
	lg.Printf("gold resale done (seller paid 60 gold)")

	// withdrawal: lock -> admin approve -> mock send -> sent
	to := "EQDtFpEwcFAEcRe5mLVh2N6C0x-_hJEM7W61_JLnSF74p4q2"
	var wd c.Withdrawal
	if err := seller.http("POST", "/api/v1/ton/withdraw", map[string]any{"to_address": to, "amount": 1_000_000_000, "request_id": randHex(8)}, &wd); err != nil {
		return err
	}
	_ = seller.http("GET", "/api/v1/ton", nil, &sw)
	if sw.Locked != 1_000_000_000 {
		return fmt.Errorf("withdrawal not locked: %+v", sw)
	}
	if _, err := bus.Request[c.Withdrawal](ctx, nb, c.AssetWithdrawalReview, c.WithdrawalReview{ID: wd.ID, Approve: true, Reviewer: "e2e"}); err != nil {
		return err
	}
	if err := seller.waitFor(func() bool {
		_ = seller.http("GET", "/api/v1/ton", nil, &sw)
		return len(sw.Withdrawals) > 0 && sw.Withdrawals[0].Status == "sent"
	}, 40*time.Second); err != nil {
		return fmt.Errorf("withdrawal never sent: %+v", sw.Withdrawals)
	}
	if sw.Locked != 0 || sw.Balance != wantSeller-1_000_000_000 {
		return fmt.Errorf("balances after withdrawal wrong: %+v", sw)
	}
	lg.Printf("withdrawal sent (tx %s...)", sw.Withdrawals[0].TxHash[:12])

	// provenance + Merkle proof of the sale
	var tv c.TokenView
	if err := seller.http("GET", fmt.Sprintf("/api/v1/tokens/%d", tok.ID), nil, &tv); err != nil {
		return err
	}
	kinds := []string{}
	var saleHash string
	for _, t := range tv.Provenance {
		kinds = append(kinds, t.Kind)
		if t.Kind == "SALE" && saleHash == "" {
			saleHash = t.Hash
		}
	}
	lg.Printf("provenance: %s", strings.Join(kinds, " > "))
	var pr c.ProofResp
	if err := seller.waitFor(func() bool {
		_ = seller.http("GET", "/api/v1/chain/tx/"+saleHash, nil, &pr)
		return pr.Valid
	}, 20*time.Second); err != nil {
		return fmt.Errorf("sale tx never proven: %+v", pr)
	}
	lg.Printf("sale proven in block %d with a %d-step Merkle proof", pr.Block.Height, len(pr.Proof))
	return nil
}

// loginAndSelect performs login, character creation and selection.
func (b *bot) loginAndSelect() error {
	var login struct {
		Token   string    `json:"token"`
		Account c.Account `json:"account"`
	}
	if err := b.http("POST", "/api/v1/auth/dev", map[string]string{"username": b.name}, &login); err != nil {
		return err
	}
	b.token, b.account = login.Token, login.Account.ID
	if err := b.http("POST", "/api/v1/characters", map[string]string{"name": b.name}, &b.char); err != nil {
		return err
	}
	var sess struct {
		Token string `json:"token"`
	}
	if err := b.http("POST", "/api/v1/session/character", map[string]string{"character_id": b.char.ID}, &sess); err != nil {
		return err
	}
	b.token = sess.Token
	return b.waitFor(func() bool {
		var inv c.InventoryResp
		_ = b.http("GET", "/api/v1/inventory", nil, &inv)
		return len(inv.Items) >= 2
	}, 10*time.Second)
}
