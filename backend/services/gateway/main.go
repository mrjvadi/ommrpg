// gateway is the only public entry point. It authenticates Telegram Mini
// App users, issues sessions and Centrifugo tokens, exposes the REST API,
// serves as Centrifugo's RPC proxy for realtime actions (move, attack,
// interact) and proxies sprite requests to sprite-service.
package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/auth"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
	"github.com/mrjvadi/ommrpg/backend/pkg/zones"
)

type settings struct {
	BotToken      string
	InitMaxAge    time.Duration
	DevLogin      bool
	CFTokenSecret string
	CFPublicURL   string
	CFProxySecret string
	RateLimit     int
}

func main() {
	s := svc.New("gateway")
	cfg := settings{
		BotToken:      config.String("TELEGRAM_BOT_TOKEN", ""),
		InitMaxAge:    config.Duration("TELEGRAM_INIT_MAX_AGE", 24*time.Hour),
		DevLogin:      config.Bool("DEV_LOGIN", false),
		CFTokenSecret: config.String("CENTRIFUGO_TOKEN_SECRET", "dev-centrifugo-token-secret"),
		CFPublicURL:   config.String("CENTRIFUGO_PUBLIC_URL", "/connection/websocket"),
		CFProxySecret: config.String("CENTRIFUGO_PROXY_SECRET", "dev-proxy-secret"),
		RateLimit:     config.Int("RATE_LIMIT_PER_SECOND", 25),
	}
	if cfg.BotToken == "" && !cfg.DevLogin {
		s.Log.Warn("TELEGRAM_BOT_TOKEN is empty and DEV_LOGIN is off: nobody can log in")
	}
	rdb, err := store.Dragonfly(s.Ctx, config.String("DRAGONFLY_URL", "redis://localhost:6379/0"))
	if err != nil {
		s.Fatal("dragonfly", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "gateway", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	spriteURL, err := url.Parse(config.String("SPRITE_URL", "http://localhost:8090"))
	if err != nil {
		s.Fatal("sprite url", err)
	}
	a := &app{
		cfg: cfg, bus: b, rdb: rdb, zones: zones.New(b), log: s.Log,
		sessions: auth.NewIssuer(config.String("SESSION_SECRET", "dev-session-secret"), config.Duration("SESSION_TTL", 7*24*time.Hour)),
		sprites:  httputil.NewSingleHostReverseProxy(spriteURL),
	}
	srv := &http.Server{
		Addr:              config.String("HTTP_ADDR", ":8080"),
		Handler:           httpx.Middleware(a.routes(), config.List("CORS_ORIGINS", nil)),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	s.Ready()
	if err := httpx.Serve(s.Ctx, srv); err != nil && err != http.ErrServerClosed {
		s.Fatal("http", err)
	}
}
