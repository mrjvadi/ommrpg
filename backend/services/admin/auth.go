package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/auth"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/hot"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
)

type app struct {
	db       *pgxpool.Pool
	rdb      *redis.Client
	bus      *bus.Bus
	cf       *centrifugo.Client
	log      *slog.Logger
	tokens   *auth.Issuer
	cfSecret string
	wsURL    string
	history  *ring
}

// Roles, from most to least powerful.
var roleRank = map[string]int{"owner": 4, "admin": 3, "moderator": 2, "viewer": 1}

type adminCtx struct{}

type admin struct {
	Name string
	Role string
}

func adminOf(r *http.Request) admin { return r.Context().Value(adminCtx{}).(admin) }

// bootstrap creates the first owner. Without a configured password a random
// one is generated and logged exactly once.
func (a *app) bootstrap(ctx context.Context, user, pass string) error {
	var n int
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM admin_users`).Scan(&n); err != nil || n > 0 {
		return err
	}
	if pass == "" {
		b := make([]byte, 9)
		_, _ = rand.Read(b)
		pass = hex.EncodeToString(b)
		a.log.Warn("created the first admin with a random password - change it", "user", user, "password", pass)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(ctx, `INSERT INTO admin_users (username, password_hash, role) VALUES ($1,$2,'owner') ON CONFLICT DO NOTHING`, user, string(h))
	return err
}

// clientIP trusts X-Real-IP, which the edge proxy (deploy/nginx) overwrites
// with the peer address; X-Forwarded-For is client-controlled and ignored.
func clientIP(r *http.Request) string {
	if f := strings.TrimSpace(r.Header.Get("X-Real-IP")); f != "" {
		return f
	}
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	return h
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	// brute force protection: GCRA in Dragonfly, per IP and per username
	for _, k := range []string{"adminlogin:ip:" + clientIP(r), "adminlogin:user:" + body.Username} {
		if l, err := hot.Allow(r.Context(), a.rdb, k, 5, 60_000, 5, 1); err == nil && !l.Allowed {
			httpx.Error(w, apperr.New(apperr.RateLimited, "too many attempts, retry in %ds", l.RetryAfter/1000+1))
			return
		}
	}
	var hash, role string
	var disabled bool
	err := a.db.QueryRow(r.Context(), `SELECT password_hash, role, disabled FROM admin_users WHERE username=$1`, body.Username).Scan(&hash, &role, &disabled)
	if errors.Is(err, pgx.ErrNoRows) || disabled || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		a.audit(r.Context(), body.Username, "login", "", nil, errors.New("bad credentials"))
		httpx.Error(w, apperr.New(apperr.Unauthorized, "wrong username or password"))
		return
	}
	if err != nil {
		httpx.Error(w, err)
		return
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE admin_users SET last_login_at=now() WHERE username=$1`, body.Username)
	tok, exp, err := a.tokens.Issue(auth.Session{AccountID: "admin:" + body.Username, CharacterID: role})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	rt, err := auth.CentrifugoToken(a.cfSecret, "admin:"+body.Username, map[string]any{"admin": body.Username},
		[]string{"admin:feed", "admin:metrics"}, 12*time.Hour)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	a.audit(r.Context(), body.Username, "login", "", nil, nil)
	httpx.JSON(w, 200, map[string]any{"token": tok, "expires_at": exp, "username": body.Username, "role": role,
		"realtime": map[string]string{"url": a.wsURL, "token": rt}})
}

// guard authenticates and enforces a minimum role.
func (a *app) guard(minRole string, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, err := a.tokens.Verify(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil || !strings.HasPrefix(s.AccountID, "admin:") {
			httpx.Error(w, apperr.New(apperr.Unauthorized, "login required"))
			return
		}
		ad := admin{Name: strings.TrimPrefix(s.AccountID, "admin:"), Role: s.CharacterID}
		var role string
		var disabled bool
		if err := a.db.QueryRow(r.Context(), `SELECT role, disabled FROM admin_users WHERE username=$1`, ad.Name).Scan(&role, &disabled); err != nil || disabled {
			httpx.Error(w, apperr.New(apperr.Unauthorized, "account disabled"))
			return
		}
		ad.Role = role // role changes apply immediately
		if roleRank[ad.Role] < roleRank[minRole] {
			httpx.Error(w, apperr.New(apperr.Forbidden, "requires role %s", minRole))
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), adminCtx{}, ad)))
	})
}

func (a *app) audit(ctx context.Context, who, action, target string, details any, err error) {
	var raw []byte
	if details != nil {
		raw, _ = json.Marshal(details)
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_, _ = a.db.Exec(ctx, `INSERT INTO audit_log (admin, action, target, details, ok, error) VALUES ($1,$2,NULLIF($3,''),$4,$5,NULLIF($6,''))`,
		who, action, target, raw, err == nil, msg)
	_ = a.cf.Publish(ctx, "admin:feed", map[string]any{"kind": "audit", "admin": who, "action": action, "target": target, "ok": err == nil, "time": time.Now().UTC()})
}
