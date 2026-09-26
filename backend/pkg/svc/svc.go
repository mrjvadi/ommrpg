// Package svc bootstraps a microservice: logging, signal handling and the
// Kubernetes-style health endpoints.
package svc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/config"
)

// Service is the running process context.
type Service struct {
	Name  string
	Cfg   config.Common
	Log   *slog.Logger
	Ctx   context.Context
	ready atomic.Bool
	stop  context.CancelFunc
}

// New creates the service, installs the logger and signal handling.
func New(name string) *Service {
	cfg := config.LoadCommon(name)
	lvl := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})).With("service", name)
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	s := &Service{Name: name, Cfg: cfg, Log: log, Ctx: ctx, stop: stop}
	go s.serveHealth()
	return s
}

// Ready flips the readiness probe.
func (s *Service) Ready() { s.ready.Store(true); s.Log.Info("ready") }

// Fatal logs and exits.
func (s *Service) Fatal(msg string, err error) {
	s.Log.Error(msg, "err", err)
	os.Exit(1)
}

// Wait blocks until SIGINT/SIGTERM.
func (s *Service) Wait() {
	<-s.Ctx.Done()
	s.ready.Store(false)
	s.Log.Info("shutting down")
}

func (s *Service) serveHealth() {
	if s.Cfg.HealthAddr == "" || s.Cfg.HealthAddr == "off" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if s.ready.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := &http.Server{Addr: s.Cfg.HealthAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-s.Ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.Log.Warn("health server stopped", "err", err)
	}
}
