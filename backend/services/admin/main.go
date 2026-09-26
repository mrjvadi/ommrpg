// admin-service powers the real-time web admin panel: admin accounts with
// roles, an audit log of every action, player/economy/market/withdrawal
// management, the OMM Chain explorer, and live metrics + event feed pushed
// through Centrifugo. The panel itself (web/) is embedded and served here.
package main

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/auth"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

//go:embed migrations/*.sql
var migrations embed.FS

//go:embed web
var webFS embed.FS

func main() {
	s := svc.New("admin")
	db, err := store.Postgres(s.Ctx, config.String("POSTGRES_DSN", "postgres://ommrpg:ommrpg@localhost:5432/admin?sslmode=disable"))
	if err != nil {
		s.Fatal("postgres", err)
	}
	sub, _ := fs.Sub(migrations, "migrations")
	if err := store.Migrate(s.Ctx, db, sub, "admin"); err != nil {
		s.Fatal("migrate", err)
	}
	rdb, err := store.Dragonfly(s.Ctx, config.String("DRAGONFLY_URL", "redis://localhost:6379/0"))
	if err != nil {
		s.Fatal("dragonfly", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "admin", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	if err := b.EnsureStreams(s.Ctx); err != nil {
		s.Fatal("streams", err)
	}
	a := &app{
		db: db, rdb: rdb, bus: b, log: s.Log,
		cf:       centrifugo.New(config.String("CENTRIFUGO_API_URL", "http://localhost:8000"), config.String("CENTRIFUGO_API_KEY", "dev-api-key")),
		tokens:   auth.NewIssuer(config.String("ADMIN_SECRET", "dev-admin-secret"), 12*time.Hour),
		cfSecret: config.String("CENTRIFUGO_TOKEN_SECRET", "dev-centrifugo-token-secret"),
		wsURL:    config.String("ADMIN_WS_URL", "/connection/websocket"),
		history:  newRing(300),
	}
	if err := a.bootstrap(s.Ctx, config.String("ADMIN_BOOTSTRAP_USER", "admin"), config.String("ADMIN_BOOTSTRAP_PASSWORD", "")); err != nil {
		s.Fatal("bootstrap", err)
	}
	if err := b.ConsumeNew(s.Ctx, "admin-feed", []string{"game.>"}, a.onEvent); err != nil {
		s.Fatal("feed", err)
	}
	go a.metricsLoop(s.Ctx)
	web, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	a.routes(mux)
	mux.Handle("GET /admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(web))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/admin/", http.StatusFound) })
	srv := &http.Server{Addr: config.String("HTTP_ADDR", ":8095"), Handler: httpx.Middleware(mux, config.List("CORS_ORIGINS", nil)), ReadHeaderTimeout: 5 * time.Second}
	s.Ready()
	if err := httpx.Serve(s.Ctx, srv); err != nil && err != http.ErrServerClosed {
		s.Fatal("http", err)
	}
}
