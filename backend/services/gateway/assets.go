package main

import (
	"net/http"
	"strconv"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
)

// Routes for NFTs (vault), the marketplace, the TON wallet and OMM Chain.
func (a *app) assetRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/vault", a.authed(false, a.vaultList))
	mux.Handle("POST /api/v1/vault/mint", a.authed(true, a.vaultMint))
	mux.Handle("POST /api/v1/vault/{token}/claim", a.authed(true, a.vaultClaim))
	mux.Handle("POST /api/v1/vault/{token}/deposit", a.authed(false, a.vaultDeposit))
	mux.Handle("POST /api/v1/vault/{token}/burn", a.authed(false, a.vaultBurn))
	mux.Handle("POST /api/v1/vault/{token}/list", a.authed(false, a.vaultListForSale))
	mux.Handle("GET /api/v1/market", a.authed(false, a.marketList))
	mux.Handle("POST /api/v1/market/{listing}/buy", a.authed(false, a.marketBuy))
	mux.Handle("POST /api/v1/market/{listing}/cancel", a.authed(false, a.marketCancel))
	mux.Handle("GET /api/v1/ton", a.authed(false, a.tonWallet))
	mux.Handle("POST /api/v1/ton/withdraw", a.authed(false, a.tonWithdraw))
	// public, verifiable chain data
	mux.HandleFunc("GET /api/v1/chain", a.chainInfo)
	mux.HandleFunc("GET /api/v1/chain/blocks", a.chainBlocks)
	mux.HandleFunc("GET /api/v1/chain/blocks/{height}", a.chainBlock)
	mux.HandleFunc("GET /api/v1/chain/tx/{hash}", a.chainProof)
	mux.HandleFunc("GET /api/v1/tokens/{token}", a.tokenView)
}

func pathID(r *http.Request, k string) (int64, error) {
	v, err := strconv.ParseInt(r.PathValue(k), 10, 64)
	if err != nil || v <= 0 {
		return 0, apperr.New(apperr.Invalid, "bad %s", k)
	}
	return v, nil
}

func (a *app) vaultList(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.TokensResp](r.Context(), a.bus, c.AssetTokens, c.TokenReq{AccountID: sessionOf(r).AccountID})
	reply(w, v, err)
}

func (a *app) vaultMint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemID    string `json:"item_id"`
		RequestID string `json:"request_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	v, err := bus.Request[c.Token](r.Context(), a.bus, c.AssetMint, c.TokenReq{AccountID: s.AccountID, CharacterID: s.CharacterID, ItemID: body.ItemID, RequestID: body.RequestID})
	reply(w, v, err)
}

func (a *app) tokenAction(w http.ResponseWriter, r *http.Request, subject string) {
	id, err := pathID(r, "token")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	v, err := bus.Request[c.Token](r.Context(), a.bus, subject, c.TokenReq{AccountID: s.AccountID, CharacterID: s.CharacterID, TokenID: id})
	if err == nil && subject != c.AssetBurn {
		a.lookChanged(r.Context(), s.CharacterID)
	}
	reply(w, v, err)
}

func (a *app) vaultClaim(w http.ResponseWriter, r *http.Request) { a.tokenAction(w, r, c.AssetClaim) }
func (a *app) vaultDeposit(w http.ResponseWriter, r *http.Request) {
	a.tokenAction(w, r, c.AssetDeposit)
}
func (a *app) vaultBurn(w http.ResponseWriter, r *http.Request) { a.tokenAction(w, r, c.AssetBurn) }

func (a *app) vaultListForSale(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "token")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	var body struct {
		Currency  string `json:"currency"`
		Price     int64  `json:"price"`
		RequestID string `json:"request_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	v, err := bus.Request[c.Listing](r.Context(), a.bus, c.AssetList, c.ListReq{AccountID: s.AccountID, CharacterID: s.CharacterID, TokenID: id,
		Currency: body.Currency, Price: body.Price, RequestID: s.AccountID + ":" + body.RequestID})
	reply(w, v, err)
}

func (a *app) marketList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	v, err := bus.Request[c.MarketResp](r.Context(), a.bus, c.AssetMarket, c.MarketReq{
		AccountID: sessionOf(r).AccountID, Currency: q.Get("currency"), Slot: q.Get("slot"), Rarity: q.Get("rarity"),
		Sort: q.Get("sort"), Mine: q.Get("mine") == "1", Limit: limit, Offset: offset,
	})
	reply(w, v, err)
}

func (a *app) marketBuy(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "listing")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	rid, err := requestID(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	v, err := bus.Request[c.Listing](r.Context(), a.bus, c.AssetBuy, c.BuyReq{AccountID: s.AccountID, CharacterID: s.CharacterID, ListingID: id, RequestID: s.AccountID + ":" + rid})
	reply(w, v, err)
}

func (a *app) marketCancel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "listing")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	v, err := bus.Request[c.Listing](r.Context(), a.bus, c.AssetCancel, c.BuyReq{AccountID: sessionOf(r).AccountID, ListingID: id})
	reply(w, v, err)
}

func (a *app) tonWallet(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.TonWallet](r.Context(), a.bus, c.AssetWallet, c.TokenReq{AccountID: sessionOf(r).AccountID})
	reply(w, v, err)
}

func (a *app) tonWithdraw(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To        string `json:"to_address"`
		Amount    int64  `json:"amount"`
		RequestID string `json:"request_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	if err := a.rateLimit(r.Context(), "withdraw:"+s.AccountID, 1); err != nil {
		httpx.Error(w, err)
		return
	}
	v, err := bus.Request[c.Withdrawal](r.Context(), a.bus, c.AssetWithdraw, c.WithdrawReq{AccountID: s.AccountID, To: body.To, Amount: body.Amount, RequestID: s.AccountID + ":" + body.RequestID})
	reply(w, v, err)
}

func (a *app) chainInfo(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.ChainInfoResp](r.Context(), a.bus, c.ChainInfo, struct{}{})
	reply(w, v, err)
}

func (a *app) chainBlocks(w http.ResponseWriter, r *http.Request) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	v, err := bus.Request[c.BlocksResp](r.Context(), a.bus, c.ChainBlocks, c.BlocksReq{Before: before, Limit: 20})
	reply(w, v, err)
}

func (a *app) chainBlock(w http.ResponseWriter, r *http.Request) {
	h, err := strconv.ParseInt(r.PathValue("height"), 10, 64)
	if err != nil {
		httpx.Error(w, apperr.New(apperr.Invalid, "bad height"))
		return
	}
	v, err := bus.Request[c.Block](r.Context(), a.bus, c.ChainBlock, c.BlocksReq{Height: h})
	reply(w, v, err)
}

func (a *app) chainProof(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.ProofResp](r.Context(), a.bus, c.ChainProof, c.ProofReq{TxHash: r.PathValue("hash")})
	reply(w, v, err)
}

func (a *app) tokenView(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "token")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	v, err := bus.Request[c.TokenView](r.Context(), a.bus, c.AssetToken, c.TokenReq{TokenID: id})
	reply(w, v, err)
}
