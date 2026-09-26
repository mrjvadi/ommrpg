package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/auth"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
	"github.com/mrjvadi/ommrpg/backend/pkg/hot"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

type app struct {
	cfg      settings
	bus      *bus.Bus
	rdb      *redis.Client
	zones    *zones.Resolver
	sessions *auth.Issuer
	sprites  *httputil.ReverseProxy
	log      *slog.Logger
}

type ctxKey struct{}

func sessionOf(r *http.Request) auth.Session { return r.Context().Value(ctxKey{}).(auth.Session) }

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /api/v1/config", a.publicConfig)
	mux.HandleFunc("POST /api/v1/auth/telegram", a.loginTelegram)
	mux.HandleFunc("POST /api/v1/auth/dev", a.loginDev)

	// account-level (no character selected yet)
	mux.Handle("GET /api/v1/me", a.authed(false, a.me))
	mux.Handle("POST /api/v1/characters", a.authed(false, a.createCharacter))
	mux.Handle("POST /api/v1/session/character", a.authed(false, a.selectCharacter))
	mux.Handle("GET /api/v1/settings/{key}", a.authed(false, a.getSetting))
	mux.Handle("PUT /api/v1/settings/{key}", a.authed(false, a.putSetting))

	// public game data
	mux.Handle("GET /api/v1/characters/{id}/public", a.authed(false, a.publicCharacter))
	mux.Handle("GET /api/v1/worlds/{id}", a.authed(false, a.worldInfo))
	mux.Handle("GET /api/v1/worlds/{id}/species", a.authed(false, a.species))
	mux.Handle("GET /api/v1/worlds/{id}/chunks/{cx}/{cy}", a.authed(false, a.chunk))
	mux.Handle("GET /api/v1/legend", a.authed(false, a.legend))
	mux.Handle("GET /api/v1/history", a.authed(false, a.history))
	mux.Handle("GET /api/v1/history/firsts", a.authed(false, a.firsts))

	// character-scoped
	mux.Handle("GET /api/v1/character", a.authed(true, a.character))
	mux.Handle("POST /api/v1/character/allocate", a.authed(true, a.allocate))
	mux.Handle("GET /api/v1/inventory", a.authed(true, a.inventory))
	mux.Handle("POST /api/v1/inventory/{item}/equip", a.authed(true, a.equip))
	mux.Handle("POST /api/v1/inventory/unequip", a.authed(true, a.unequip))
	mux.Handle("POST /api/v1/inventory/{item}/enhance", a.authed(true, a.enhance))
	mux.Handle("POST /api/v1/inventory/{item}/salvage", a.authed(true, a.salvage))

	// sprites: public, immutable, cacheable
	mux.Handle("GET /api/v1/sprites/", http.StripPrefix("/api", a.sprites))
	mux.Handle("POST /api/v1/sprites/character", http.StripPrefix("/api", a.sprites))

	a.assetRoutes(mux)

	// Centrifugo RPC proxy (internal network only, shared-secret protected)
	mux.HandleFunc("POST /centrifugo/rpc", a.rpcProxy)
	return mux
}

// ---------------------------------------------------------------- auth

func (a *app) authed(needCharacter bool, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		s, err := a.sessions.Verify(tok)
		if err != nil {
			httpx.Error(w, apperr.New(apperr.Unauthorized, "login required"))
			return
		}
		if n, _ := a.rdb.Exists(r.Context(), c.BannedKey(s.AccountID)).Result(); n > 0 {
			httpx.Error(w, apperr.New(apperr.Forbidden, "this account is banned"))
			return
		}
		if needCharacter && s.CharacterID == "" {
			httpx.Error(w, apperr.New(apperr.Forbidden, "select a character first"))
			return
		}
		if err := a.rateLimit(r.Context(), "http:"+s.AccountID, a.cfg.RateLimit); err != nil {
			httpx.Error(w, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, s)))
	})
}

// rateLimit runs GCRA inside Dragonfly (pkg/hot gcra.lua): a smooth rate
// of perSecond with a burst of the same size, shared by every replica.
func (a *app) rateLimit(ctx context.Context, key string, perSecond int) error {
	l, err := hot.Allow(ctx, a.rdb, "rl:"+key, perSecond, 1000, perSecond, 1)
	if err != nil {
		return nil // fail open: rate limiting must not take the game down
	}
	if !l.Allowed {
		return apperr.New(apperr.RateLimited, "slow down (retry in %dms)", l.RetryAfter)
	}
	return nil
}

func (a *app) publicConfig(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, 200, map[string]any{
		"realtime_url": a.cfg.CFPublicURL,
		"dev_login":    a.cfg.DevLogin,
		"telegram":     a.cfg.BotToken != "",
	})
}

type loginResp struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Account   c.Account `json:"account"`
}

func (a *app) issue(w http.ResponseWriter, acc c.Account, charID string) {
	tok, exp, err := a.sessions.Issue(auth.Session{AccountID: acc.ID, TelegramID: acc.TelegramID, CharacterID: charID})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, loginResp{Token: tok, ExpiresAt: exp, Account: acc})
}

func (a *app) loginTelegram(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	d, err := auth.ValidateInitData(body.InitData, a.cfg.BotToken, a.cfg.InitMaxAge, time.Now())
	if err != nil {
		httpx.Error(w, apperr.New(apperr.Unauthorized, "%v", err))
		return
	}
	acc, err := bus.Request[c.Account](r.Context(), a.bus, c.IdentityTelegramLogin, c.TelegramLoginReq{User: d.User})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	a.issue(w, acc, "")
}

func (a *app) loginDev(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.DevLogin {
		httpx.Error(w, apperr.New(apperr.Forbidden, "dev login is disabled"))
		return
	}
	var body c.DevLoginReq
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	acc, err := bus.Request[c.Account](r.Context(), a.bus, c.IdentityDevLogin, body)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	a.issue(w, acc, "")
}

// ---------------------------------------------------------------- account

func reply[T any](w http.ResponseWriter, v T, err error) {
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, v)
}

func (a *app) me(w http.ResponseWriter, r *http.Request) {
	s := sessionOf(r)
	acc, err := bus.Request[c.Account](r.Context(), a.bus, c.IdentityGet, c.AccountReq{AccountID: s.AccountID})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	chars, err := bus.Request[c.CharacterListResp](r.Context(), a.bus, c.CharacterList, c.AccountReq{AccountID: s.AccountID})
	reply(w, map[string]any{"account": acc, "characters": chars.Characters, "character_id": s.CharacterID}, err)
}

func (a *app) createCharacter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string    `json:"name"`
		BodyType   string    `json:"body_type"`
		Appearance *c.Recipe `json:"appearance"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	ch, err := bus.Request[c.Character](r.Context(), a.bus, c.CharacterCreate, c.CreateCharacterReq{
		AccountID: sessionOf(r).AccountID, Name: body.Name, BodyType: body.BodyType, Appearance: body.Appearance})
	reply(w, ch, err)
}

// selectCharacter binds a character to the session and returns everything
// the client needs to enter the world, including the realtime token.
func (a *app) selectCharacter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CharacterID string `json:"character_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	ctx := r.Context()
	ch, err := bus.Request[c.Character](ctx, a.bus, c.CharacterGet, c.CharacterReq{CharacterID: body.CharacterID, AccountID: s.AccountID})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	pos, err := bus.Request[c.Position](ctx, a.bus, c.PresenceEnter, c.CharacterReq{CharacterID: ch.ID})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s.CharacterID = ch.ID
	a.rdb.Set(ctx, c.CharAccountKey(ch.ID), s.AccountID, 30*24*time.Hour)
	tok, exp, err := a.sessions.Issue(s)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	rt, err := auth.CentrifugoToken(a.cfg.CFTokenSecret, ch.ID, map[string]any{"name": ch.Name},
		[]string{centrifugo.PersonalChannel(ch.ID), centrifugo.NewsChannel}, 24*time.Hour)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	wi, err := a.zones.World(ctx, ch.WorldID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{
		"token": tok, "expires_at": exp, "character": ch, "position": pos,
		"world":    wi,
		"realtime": map[string]any{"url": a.cfg.CFPublicURL, "token": rt},
		"tileset":  "/api/v1/sprites/tileset.png?seed=" + strconv.FormatUint(wi.Seed, 10),
	})
}

func (a *app) getSetting(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.SettingsValue](r.Context(), a.bus, c.IdentitySettingsGet, c.SettingsGetReq{AccountID: sessionOf(r).AccountID, Key: r.PathValue("key")})
	reply(w, v, err)
}

func (a *app) putSetting(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&raw); err != nil {
		httpx.Error(w, apperr.New(apperr.Invalid, "invalid JSON"))
		return
	}
	v, err := bus.Request[c.SettingsValue](r.Context(), a.bus, c.IdentitySettingsPut, c.SettingsPutReq{AccountID: sessionOf(r).AccountID, Key: r.PathValue("key"), Value: raw})
	reply(w, v, err)
}

// ---------------------------------------------------------------- world data

func (a *app) publicCharacter(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.PublicCharacter](r.Context(), a.bus, c.CharacterPublic, c.CharacterReq{CharacterID: r.PathValue("id")})
	reply(w, v, err)
}

func pathInt(r *http.Request, k string) (int, error) {
	v, err := strconv.Atoi(r.PathValue(k))
	if err != nil {
		return 0, apperr.New(apperr.Invalid, "bad %s", k)
	}
	return v, nil
}

func (a *app) worldInfo(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	v, err := a.zones.World(r.Context(), id)
	reply(w, v, err)
}

func (a *app) species(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	v, err := bus.Request[c.SpeciesResp](r.Context(), a.bus, c.WorldSpecies, c.WorldReq{WorldID: id})
	if err == nil {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	reply(w, v, err)
}

func (a *app) chunk(w http.ResponseWriter, r *http.Request) {
	id, err1 := pathInt(r, "id")
	cx, err2 := pathInt(r, "cx")
	cy, err3 := pathInt(r, "cy")
	if err1 != nil || err2 != nil || err3 != nil {
		httpx.Error(w, apperr.New(apperr.Invalid, "bad chunk path"))
		return
	}
	raw, err := bus.Request[json.RawMessage](r.Context(), a.bus, c.WorldChunk, c.ChunkReq{WorldID: id, CX: cx, CY: cy})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(raw)
}

func (a *app) legend(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, 200, world.Legend())
}

func (a *app) history(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	v, err := bus.Request[c.HistoryResp](r.Context(), a.bus, c.HistoryRecent, c.HistoryReq{Limit: n})
	reply(w, v, err)
}

func (a *app) firsts(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	v, err := bus.Request[c.HistoryResp](r.Context(), a.bus, c.HistoryFirsts, c.HistoryReq{Limit: n})
	reply(w, v, err)
}

// ---------------------------------------------------------------- character

func (a *app) character(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.Character](r.Context(), a.bus, c.CharacterGet, c.CharacterReq{CharacterID: sessionOf(r).CharacterID})
	reply(w, v, err)
}

func (a *app) allocate(w http.ResponseWriter, r *http.Request) {
	var body c.AllocateReq
	if err := httpx.Decode(r, &body.Points); err != nil {
		httpx.Error(w, err)
		return
	}
	body.CharacterID = sessionOf(r).CharacterID
	v, err := bus.Request[c.Character](r.Context(), a.bus, c.CharacterAllocate, body)
	reply(w, v, err)
}

func (a *app) inventory(w http.ResponseWriter, r *http.Request) {
	v, err := bus.Request[c.InventoryResp](r.Context(), a.bus, c.ItemList, c.ItemActionReq{CharacterID: sessionOf(r).CharacterID})
	reply(w, v, err)
}

func (a *app) lookChanged(ctx context.Context, id string) {
	if _, err := bus.Request[struct{}](ctx, a.bus, c.PresenceLook, c.CharacterReq{CharacterID: id}); err != nil {
		a.log.Debug("look broadcast", "err", err)
	}
}

func (a *app) equip(w http.ResponseWriter, r *http.Request) {
	id := sessionOf(r).CharacterID
	v, err := bus.Request[c.InventoryResp](r.Context(), a.bus, c.ItemEquip, c.ItemActionReq{CharacterID: id, ItemID: r.PathValue("item")})
	if err == nil {
		a.lookChanged(r.Context(), id)
	}
	reply(w, v, err)
}

func (a *app) unequip(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slot string `json:"slot"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	id := sessionOf(r).CharacterID
	v, err := bus.Request[c.InventoryResp](r.Context(), a.bus, c.ItemUnequip, c.ItemActionReq{CharacterID: id, Slot: body.Slot})
	if err == nil {
		a.lookChanged(r.Context(), id)
	}
	reply(w, v, err)
}

func requestID(r *http.Request) (string, error) {
	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		return "", err
	}
	if len(body.RequestID) < 8 || len(body.RequestID) > 64 {
		return "", apperr.New(apperr.Invalid, "request_id (8-64 chars) is required")
	}
	return body.RequestID, nil
}

func (a *app) enhance(w http.ResponseWriter, r *http.Request) {
	rid, err := requestID(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	v, err := bus.Request[c.EnhanceResp](r.Context(), a.bus, c.ItemEnhance, c.ItemActionReq{CharacterID: s.CharacterID, ItemID: r.PathValue("item"), RequestID: s.CharacterID + ":" + rid})
	reply(w, v, err)
}

func (a *app) salvage(w http.ResponseWriter, r *http.Request) {
	rid, err := requestID(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s := sessionOf(r)
	v, err := bus.Request[c.SalvageResp](r.Context(), a.bus, c.ItemSalvage, c.ItemActionReq{CharacterID: s.CharacterID, ItemID: r.PathValue("item"), RequestID: s.CharacterID + ":" + rid})
	reply(w, v, err)
}

// ---------------------------------------------------------------- realtime

func (a *app) rpcProxy(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Proxy-Secret")), []byte(a.cfg.CFProxySecret)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req centrifugo.RPCRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var resp centrifugo.RPCResponse
	data, err := a.dispatch(r.Context(), req.User, req.Method, req.Data)
	if err != nil {
		e := apperr.From(err)
		if e.Code == apperr.Internal {
			a.log.Error("rpc failed", "method", req.Method, "err", err)
		}
		resp.Error = &centrifugo.RPCError{Code: e.RPCCode(), Message: string(e.Code) + ": " + e.Message}
	} else {
		resp.Result = &centrifugo.RPCResult{Data: data}
	}
	httpx.JSON(w, 200, resp)
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &v); err != nil {
			return v, apperr.New(apperr.Invalid, "bad rpc data")
		}
	}
	return v, nil
}

// dispatch routes a realtime RPC. `char` is the authenticated Centrifugo
// user, i.e. the character id from the connection token.
func (a *app) dispatch(ctx context.Context, char, method string, raw json.RawMessage) (any, error) {
	if char == "" {
		return nil, apperr.New(apperr.Unauthorized, "anonymous")
	}
	limit := 30
	if method == "move" {
		limit = 25
	}
	if err := a.rateLimit(ctx, "rpc:"+method+":"+char, limit); err != nil {
		return nil, err
	}
	if acc := a.rdb.Get(ctx, c.CharAccountKey(char)).Val(); acc != "" {
		if n, _ := a.rdb.Exists(ctx, c.BannedKey(acc)).Result(); n > 0 {
			return nil, apperr.New(apperr.Forbidden, "this account is banned")
		}
	}
	cr := c.CharacterReq{CharacterID: char}
	switch method {
	case "ping":
		return map[string]any{"t": time.Now().UnixMilli()}, nil
	case "enter":
		return bus.Request[c.Position](ctx, a.bus, c.PresenceEnter, cr)
	case "move":
		m, err := decode[c.MoveReq](raw)
		if err != nil {
			return nil, err
		}
		m.CharacterID = char
		return bus.Request[c.MoveResp](ctx, a.bus, c.PresenceMove, m)
	case "nearby":
		return bus.Request[c.NearbyResp](ctx, a.bus, c.PresenceNearby, cr)
	case "monsters":
		return bus.Request[c.MonstersResp](ctx, a.bus, c.CombatMonsters, c.MonstersReq{CharacterID: char})
	case "vitals":
		return bus.Request[c.PlayerVitals](ctx, a.bus, c.CombatPlayer, cr)
	case "attack":
		d, err := decode[struct {
			Target string `json:"target"`
		}](raw)
		if err != nil {
			return nil, err
		}
		return bus.Request[c.AttackResp](ctx, a.bus, c.CombatAttack, c.AttackReq{CharacterID: char, TargetID: d.Target})
	case "dungeon":
		return bus.Request[c.DungeonView](ctx, a.bus, c.DungeonFloor, c.DungeonReq{CharacterID: char})
	case "interact":
		d, err := decode[struct {
			X int `json:"x"`
			Y int `json:"y"`
		}](raw)
		if err != nil {
			return nil, err
		}
		return a.interact(ctx, char, d.X, d.Y)
	}
	return nil, apperr.New(apperr.NotFound, "unknown method %q", method)
}
