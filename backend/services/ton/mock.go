package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

// mockChain simulates the TON wallet for development: deposits are injected
// through NATS (ton.mock.deposit, used by the admin panel and tests) and
// sends always succeed with a fake hash.
type mockChain struct {
	mu   sync.Mutex
	lt   uint64
	deps []Deposit
}

func newMock() *mockChain { return &mockChain{lt: uint64(time.Now().UnixNano())} }

func (m *mockChain) Address() string { return "EQMOCK_OMMRPG_GAME_WALLET_DEVELOPMENT_ONLY_____" }
func (m *mockChain) Network() string { return "mock" }

func fakeHash(parts ...any) string {
	s := sha256.Sum256([]byte(fmt.Sprint(parts...)))
	return hex.EncodeToString(s[:])
}

func (m *mockChain) inject(_ context.Context, req c.MockDepositReq) (struct{}, error) {
	if req.Amount <= 0 || req.Memo == "" {
		return struct{}{}, apperr.New(apperr.Invalid, "memo and positive amount required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lt++
	sender := req.Sender
	if sender == "" {
		sender = "EQMOCK_SENDER"
	}
	m.deps = append(m.deps, Deposit{TxHash: fakeHash("dep", m.lt, req.Memo), LT: m.lt, Amount: req.Amount, Memo: req.Memo, Sender: sender})
	return struct{}{}, nil
}

func (m *mockChain) Deposits(_ context.Context, after uint64) ([]Deposit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Deposit
	for _, d := range m.deps {
		if d.LT > after {
			out = append(out, d)
		}
	}
	return out, nil
}

func (m *mockChain) Send(_ context.Context, to string, nano int64, comment string) (string, error) {
	return fakeHash("send", to, nano, comment, time.Now().UnixNano()), nil
}
