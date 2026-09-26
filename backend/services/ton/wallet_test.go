package main

import (
	"context"
	"testing"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/ton/wallet"

	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

// Key derivation and address formatting work offline.
func TestWalletFromSeedOffline(t *testing.T) {
	words := wallet.NewSeed()
	w, err := wallet.FromSeed(nil, words, wallet.V4R2)
	if err != nil {
		t.Fatal(err)
	}
	a := w.WalletAddress().String()
	if _, err := address.ParseAddr(a); err != nil {
		t.Fatalf("derived address %q does not parse: %v", a, err)
	}
	w2, _ := wallet.FromSeed(nil, words, wallet.V4R2)
	if w2.WalletAddress().String() != a {
		t.Fatal("derivation not deterministic")
	}
}

func TestMockDeposits(t *testing.T) {
	m := newMock()
	ctx := context.Background()
	if _, err := m.inject(ctx, c.MockDepositReq{Memo: "OMMABC", Amount: 5}); err != nil {
		t.Fatal(err)
	}
	d, _ := m.Deposits(ctx, 0)
	if len(d) != 1 || d[0].Memo != "OMMABC" {
		t.Fatalf("deposits %+v", d)
	}
	if again, _ := m.Deposits(ctx, d[0].LT); len(again) != 0 {
		t.Fatal("cursor not respected")
	}
}
