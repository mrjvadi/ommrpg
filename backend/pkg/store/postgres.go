// Package store connects to the databases. Each service owns its own
// PostgreSQL database/schema and runs its own embedded migrations.
package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres opens a pool, retrying while the database starts up.
func Postgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("postgres: %w", lastErr)
}

// Migrate applies *.sql files from fsys (sorted by name) exactly once each.
// A transaction-scoped advisory lock serialises concurrent replicas.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, service string) error {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "migrate:"+service); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
			return err
		}
		for _, f := range files {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, f).Scan(&exists); err != nil {
				return err
			}
			if exists {
				continue
			}
			sql, err := fs.ReadFile(fsys, f)
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(sql)) != "" {
				if _, err := tx.Exec(ctx, string(sql)); err != nil {
					return fmt.Errorf("migration %s: %w", f, err)
				}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(name) VALUES ($1)`, f); err != nil {
				return err
			}
		}
		return nil
	})
}
