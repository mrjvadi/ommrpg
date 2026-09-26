package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/zone"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
)

func (a *app) routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin/api/login", a.login)
	mux.Handle("GET /admin/api/me", a.guard("viewer", func(w http.ResponseWriter, r *http.Request) {
		ad := adminOf(r)
		httpx.JSON(w, 200, map[string]string{"username": ad.Name, "role": ad.Role})
	}))
	mux.Handle("GET /admin/api/dashboard", a.guard("viewer", a.dashboard))
	mux.Handle("GET /admin/api/players", a.guard("viewer", a.players))
	mux.Handle("GET /admin/api/players/{id}", a.guard("viewer", a.player))
	mux.Handle("POST /admin/api/players/{id}/grant", a.guard("admin", a.grant))
	mux.Handle("POST /admin/api/players/{id}/respawn", a.guard("moderator", a.respawn))
	mux.Handle("POST /admin/api/players/{id}/kick", a.guard("moderator", a.kick))
	mux.Handle("POST /admin/api/accounts/{id}/ban", a.guard("moderator", a.ban))
	mux.Handle("GET /admin/api/market", a.guard("viewer", a.market))
	mux.Handle("POST /admin/api/market/{id}/cancel", a.guard("moderator", a.cancelListing))
	mux.Handle("GET /admin/api/withdrawals", a.guard("viewer", a.withdrawals))
	mux.Handle("POST /admin/api/withdrawals/{id}/review", a.guard("admin", a.review))
	mux.Handle("GET /admin/api/policy", a.guard("viewer", a.policyGet))
	mux.Handle("PUT /admin/api/policy", a.guard("admin", a.policyPut))
	mux.Handle("GET /admin/api/chain", a.guard("viewer", a.chain))
	mux.Handle("GET /admin/api/chain/blocks/{h}", a.guard("viewer", a.chainBlock))
	mux.Handle("GET /admin/api/chain/tx/{hash}", a.guard("viewer", a.chainTx))
	mux.Handle("GET /admin/api/tokens/{id}", a.guard("viewer", a.token))
	mux.Handle("POST /admin/api/announce", a.guard("moderator", a.announce))
	mux.Handle("POST /admin/api/ton/mock-deposit", a.guard("admin", a.mockDeposit))
	mux.Handle("GET /admin/api/audit", a.guard("admin", a.auditList))
	mux.Handle("GET /admin/api/admins", a.guard("owner", a.admins))
	mux.Handle("POST /admin/api/admins", a.guard("owner", a.adminSave))
}

func reply[T any](w http.ResponseWriter, v T, err error) {
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, v)
}

func opID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return "admin:" + hex.EncodeToString(b)
}

// act runs an audited action and replies with its result.
func act[T any](a *app, w http.ResponseWriter, r *http.Request, action, target string, details any, fn func(ctx context.Context) (T, error)) {
	v, err := fn(r.Context())
	a.audit(r.Context(), adminOf(r).Name, action, target, details, err)
	reply(w, v, err)
}

func (a *app) players(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.CharacterListResp](r.Context(), a.bus, c.CharacterSearch, c.SearchReq{Query: r.URL.Query().Get("q"), Limit: 50})
	reply(w, v, err)
}

type playerView struct {
	Character c.Character      `json:"character"`
	Account   *c.Account       `json:"account,omitempty"`
	Inventory *c.InventoryResp `json:"inventory,omitempty"`
	Position  *c.Position      `json:"position,omitempty"`
	Vitals    *c.PlayerVitals  `json:"vitals,omitempty"`
	Ton       *c.TonWallet     `json:"ton,omitempty"`
	Tokens    []c.Token        `json:"tokens"`
}

func (a *app) player(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	ch, err := bus.Request[c.Character](ctx, a.bus, c.CharacterGet, c.CharacterReq{CharacterID: id})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	v := playerView{Character: ch, Tokens: []c.Token{}}
	if acc, err := bus.Request[c.Account](ctx, a.bus, c.IdentityGet, c.AccountReq{AccountID: ch.AccountID}); err == nil {
		v.Account = &acc
	}
	if inv, err := bus.Request[c.InventoryResp](ctx, a.bus, c.ItemList, c.ItemActionReq{CharacterID: id}); err == nil {
		v.Inventory = &inv
	}
	if p, err := bus.Request[c.Position](ctx, a.bus, c.PresenceGet, c.CharacterReq{CharacterID: id}); err == nil {
		v.Position = &p
	}
	if vt, err := bus.Request[c.PlayerVitals](ctx, a.bus, c.CombatPlayer, c.CharacterReq{CharacterID: id}); err == nil {
		v.Vitals = &vt
	}
	if tw, err := bus.Request[c.TonWallet](ctx, a.bus, c.AssetWallet, c.TokenReq{AccountID: ch.AccountID}); err == nil {
		v.Ton = &tw
	}
	if tk, err := bus.Request[c.TokensResp](ctx, a.bus, c.AssetTokens, c.TokenReq{AccountID: ch.AccountID}); err == nil {
		v.Tokens = tk.Tokens
	}
	httpx.JSON(w, 200, v)
}

func (a *app) grant(w http.ResponseWriter, r *http.Request) {
	var body c.AdminGrantReq
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	if body.Reason == "" {
		httpx.Error(w, apperr.New(apperr.Invalid, "a reason is required"))
		return
	}
	body.CharacterID = r.PathValue("id")
	body.OpID = opID()
	act(a, w, r, "grant", body.CharacterID, body, func(ctx context.Context) (c.InventoryResp, error) {
		return bus.Request[c.InventoryResp](ctx, a.bus, c.ItemAdminGrant, body)
	})
}

func (a *app) respawn(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	act(a, w, r, "respawn", id, nil, func(ctx context.Context) (c.Position, error) {
		ch, err := bus.Request[c.Character](ctx, a.bus, c.CharacterGet, c.CharacterReq{CharacterID: id})
		if err != nil {
			return c.Position{}, err
		}
		wd, err := bus.Request[c.WorldInfo](ctx, a.bus, c.WorldGet, c.WorldReq{WorldID: ch.WorldID})
		if err != nil {
			return c.Position{}, err
		}
		return bus.Request[c.Position](ctx, a.bus, c.PresenceTeleport, c.TeleportReq{CharacterID: id, Zone: zone.World(wd.ID).String(),
			X: float64(wd.SpawnX) + 0.5, Y: float64(wd.SpawnY) + 1.5, Reason: "admin"})
	})
}

func (a *app) kick(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	act(a, w, r, "kick", id, nil, func(ctx context.Context) (map[string]bool, error) {
		return map[string]bool{"ok": true}, a.cf.Disconnect(ctx, id)
	})
}

func (a *app) ban(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Banned bool   `json:"banned"`
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	id := r.PathValue("id")
	act(a, w, r, map[bool]string{true: "ban", false: "unban"}[body.Banned], id, body, func(ctx context.Context) (c.Account, error) {
		acc, err := bus.Request[c.Account](ctx, a.bus, c.IdentityBan, c.BanReq{AccountID: id, Banned: body.Banned, Reason: body.Reason})
		if err == nil && body.Banned {
			if chars, err := bus.Request[c.CharacterListResp](ctx, a.bus, c.CharacterList, c.AccountReq{AccountID: id}); err == nil {
				for _, ch := range chars.Characters {
					_ = a.cf.Disconnect(ctx, ch.ID)
				}
			}
		}
		return acc, err
	})
}

func (a *app) market(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "active"
	}
	v, err := bus.Request[c.MarketResp](r.Context(), a.bus, c.AssetMarket, c.MarketReq{Status: status, Currency: q.Get("currency"), Sort: "new", Limit: 100})
	reply(w, v, err)
}

func (a *app) cancelListing(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	act(a, w, r, "market.cancel", r.PathValue("id"), nil, func(ctx context.Context) (c.Listing, error) {
		return bus.Request[c.Listing](ctx, a.bus, c.AssetAdminCancel, c.BuyReq{ListingID: id})
	})
}

func (a *app) withdrawals(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.WithdrawalsResp](r.Context(), a.bus, c.AssetWithdrawals, c.WithdrawalsReq{Status: r.URL.Query().Get("status"), Limit: 200})
	reply(w, v, err)
}

func (a *app) review(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	act(a, w, r, map[bool]string{true: "withdrawal.approve", false: "withdrawal.reject"}[body.Approve], r.PathValue("id"), body, func(ctx context.Context) (c.Withdrawal, error) {
		return bus.Request[c.Withdrawal](ctx, a.bus, c.AssetWithdrawalReview, c.WithdrawalReview{ID: id, Approve: body.Approve, Reviewer: adminOf(r).Name, Note: body.Note})
	})
}

func (a *app) policyGet(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.MarketPolicy](r.Context(), a.bus, c.AssetPolicyGet, struct{}{})
	reply(w, v, err)
}

func (a *app) policyPut(w http.ResponseWriter, r *http.Request) {
	var p c.MarketPolicy
	if err := httpx.Decode(r, &p); err != nil {
		httpx.Error(w, err)
		return
	}
	act(a, w, r, "policy.update", "", p, func(ctx context.Context) (c.MarketPolicy, error) {
		return bus.Request[c.MarketPolicy](ctx, a.bus, c.AssetPolicySet, p)
	})
}

func (a *app) chain(w http.ResponseWriter, r *http.Request) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	info, err := bus.Request[c.ChainInfoResp](r.Context(), a.bus, c.ChainInfo, struct{}{})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	blocks, err := bus.Request[c.BlocksResp](r.Context(), a.bus, c.ChainBlocks, c.BlocksReq{Before: before, Limit: 25})
	reply(w, map[string]any{"info": info, "blocks": blocks.Blocks}, err)
}

func (a *app) chainBlock(w http.ResponseWriter, r *http.Request) {
	h, _ := strconv.ParseInt(r.PathValue("h"), 10, 64)
	v, err := bus.Request[c.Block](r.Context(), a.bus, c.ChainBlock, c.BlocksReq{Height: h})
	reply(w, v, err)
}

func (a *app) chainTx(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.ProofResp](r.Context(), a.bus, c.ChainProof, c.ProofReq{TxHash: r.PathValue("hash")})
	reply(w, v, err)
}

func (a *app) token(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	v, err := bus.Request[c.TokenView](r.Context(), a.bus, c.AssetToken, c.TokenReq{TokenID: id})
	reply(w, v, err)
}

func (a *app) announce(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := httpx.Decode(r, &body); err != nil || len(body.Text) == 0 || len(body.Text) > 280 {
		httpx.Error(w, apperr.New(apperr.Invalid, "text (1-280 chars) required"))
		return
	}
	act(a, w, r, "announce", "", body, func(ctx context.Context) (map[string]bool, error) {
		return map[string]bool{"ok": true}, a.cf.Publish(ctx, centrifugo.NewsChannel, map[string]any{"t": "announce", "text": body.Text})
	})
}

func (a *app) mockDeposit(w http.ResponseWriter, r *http.Request) {
	var body c.MockDepositReq
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	act(a, w, r, "ton.mock_deposit", body.Memo, body, func(ctx context.Context) (struct{}, error) {
		return bus.Request[struct{}](ctx, a.bus, c.TonMockCredit, body)
	})
}

type auditRow struct {
	ID      int64           `json:"id"`
	Admin   string          `json:"admin"`
	Action  string          `json:"action"`
	Target  string          `json:"target,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Time    time.Time       `json:"time"`
}

func (a *app) auditList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id, admin, action, COALESCE(target,''), details, ok, COALESCE(error,''), created_at FROM audit_log ORDER BY id DESC LIMIT 200`)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	defer rows.Close()
	out := []auditRow{}
	for rows.Next() {
		var x auditRow
		var d []byte
		if err := rows.Scan(&x.ID, &x.Admin, &x.Action, &x.Target, &d, &x.OK, &x.Error, &x.Time); err == nil {
			if len(d) > 0 {
				x.Details = d
			}
			out = append(out, x)
		}
	}
	httpx.JSON(w, 200, map[string]any{"entries": out})
}

func (a *app) admins(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT username, role, disabled, created_at, last_login_at FROM admin_users ORDER BY created_at`)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	defer rows.Close()
	type row struct {
		Username  string     `json:"username"`
		Role      string     `json:"role"`
		Disabled  bool       `json:"disabled"`
		CreatedAt time.Time  `json:"created_at"`
		LastLogin *time.Time `json:"last_login_at"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if rows.Scan(&x.Username, &x.Role, &x.Disabled, &x.CreatedAt, &x.LastLogin) == nil {
			out = append(out, x)
		}
	}
	httpx.JSON(w, 200, map[string]any{"admins": out})
}

func (a *app) adminSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Disabled bool   `json:"disabled"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	if _, ok := roleRank[body.Role]; !ok || len(body.Username) < 3 {
		httpx.Error(w, apperr.New(apperr.Invalid, "username (3+) and a valid role are required"))
		return
	}
	if body.Username == adminOf(r).Name && (body.Disabled || body.Role != "owner") {
		httpx.Error(w, apperr.New(apperr.Invalid, "you cannot demote or disable yourself"))
		return
	}
	act(a, w, r, "admin.save", body.Username, map[string]any{"role": body.Role, "disabled": body.Disabled}, func(ctx context.Context) (map[string]bool, error) {
		if body.Password != "" {
			if len(body.Password) < 10 {
				return nil, apperr.New(apperr.Invalid, "password must be at least 10 characters")
			}
			h, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
			if err != nil {
				return nil, err
			}
			_, err = a.db.Exec(ctx, `INSERT INTO admin_users (username, password_hash, role, disabled) VALUES ($1,$2,$3,$4)
				ON CONFLICT (username) DO UPDATE SET password_hash=EXCLUDED.password_hash, role=EXCLUDED.role, disabled=EXCLUDED.disabled`,
				body.Username, string(h), body.Role, body.Disabled)
			return map[string]bool{"ok": err == nil}, err
		}
		tag, err := a.db.Exec(ctx, `UPDATE admin_users SET role=$2, disabled=$3 WHERE username=$1`, body.Username, body.Role, body.Disabled)
		if err == nil && tag.RowsAffected() == 0 {
			err = apperr.New(apperr.NotFound, "new admins need a password")
		}
		return map[string]bool{"ok": err == nil}, err
	})
}
