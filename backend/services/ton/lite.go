package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

// liteChain talks to TON lite servers (no third-party HTTP API needed).
type liteChain struct {
	api     ton.APIClientWrapped
	w       *wallet.Wallet
	network string
}

func newLite(ctx context.Context, network, seed, version, configURL string) (*liteChain, error) {
	words := strings.Fields(seed)
	if len(words) != 24 {
		return nil, fmt.Errorf("TON_WALLET_SEED must be 24 words")
	}
	if configURL == "" {
		configURL = "https://ton.org/global.config.json"
		if network == "testnet" {
			configURL = "https://ton.org/testnet-global.config.json"
		}
	}
	pool := liteclient.NewConnectionPool()
	if err := pool.AddConnectionsFromConfigUrl(ctx, configURL); err != nil {
		return nil, fmt.Errorf("lite servers: %w", err)
	}
	api := ton.NewAPIClient(pool).WithRetry()
	v := wallet.V4R2
	if strings.EqualFold(version, "v5r1") {
		v = wallet.V5R1Final
	}
	var cfg wallet.VersionConfig = v
	if v == wallet.V5R1Final {
		netID := int32(-239)
		if network == "testnet" {
			netID = -3
		}
		cfg = wallet.ConfigV5R1Final{NetworkGlobalID: netID}
	}
	w, err := wallet.FromSeed(api, words, cfg)
	if err != nil {
		return nil, err
	}
	return &liteChain{api: api, w: w, network: network}, nil
}

func (l *liteChain) Address() string { return l.w.WalletAddress().String() }
func (l *liteChain) Network() string { return l.network }

// Deposits walks the wallet's transactions backwards from the latest one
// until it reaches `after`, and returns the incoming transfers (oldest
// first). Bounced messages and transfers without a comment are skipped.
func (l *liteChain) Deposits(ctx context.Context, after uint64) ([]Deposit, error) {
	master, err := l.api.CurrentMasterchainInfo(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := l.api.GetAccount(ctx, master, l.w.WalletAddress())
	if err != nil {
		return nil, err
	}
	if !acc.IsActive || acc.LastTxLT == 0 {
		return nil, nil
	}
	var out []Deposit
	lt, hash := acc.LastTxLT, acc.LastTxHash
	for lt > after {
		txs, err := l.api.ListTransactions(ctx, l.w.WalletAddress(), 15, lt, hash)
		if err != nil || len(txs) == 0 {
			break
		}
		stop := false
		for i := len(txs) - 1; i >= 0; i-- { // newest first
			tx := txs[i]
			if tx.LT <= after {
				stop = true
				break
			}
			if tx.IO.In != nil && tx.IO.In.MsgType == tlb.MsgTypeInternal {
				in := tx.IO.In.AsInternal()
				if !in.Bounced && in.Comment() != "" {
					out = append(out, Deposit{
						TxHash: hex.EncodeToString(tx.Hash), LT: tx.LT,
						Amount: in.Amount.Nano().Int64(), Memo: strings.TrimSpace(in.Comment()), Sender: in.SrcAddr.String(),
					})
				}
			}
		}
		if stop {
			break
		}
		last := txs[0]
		lt, hash = last.PrevTxLT, last.PrevTxHash
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (l *liteChain) Send(ctx context.Context, to string, nano int64, comment string) (string, error) {
	addr, err := address.ParseAddr(to)
	if err != nil {
		return "", err
	}
	tx, _, err := l.w.TransferWaitTransaction(ctx, addr, tlb.FromNanoTON(big.NewInt(nano)), comment)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(tx.Hash), nil
}
