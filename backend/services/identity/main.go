// identity-service owns accounts (Telegram identities) and per-account
// settings such as the player's customised HUD layout.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/store"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	s := svc.New("identity")
	db, err := store.Postgres(s.Ctx, config.String("POSTGRES_DSN", "postgres://ommrpg:ommrpg@localhost:5432/identity?sslmode=disable"))
	if err != nil {
		s.Fatal("postgres", err)
	}
	sub, _ := fsSub()
	if err := store.Migrate(s.Ctx, db, sub, "identity"); err != nil {
		s.Fatal("migrate", err)
	}
	b, err := bus.Connect(s.Cfg.NATSURL, "identity", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	a := &app{db: db}
	for _, err := range []error{
		bus.Handle(b, c.IdentityTelegramLogin, a.telegramLogin),
		bus.Handle(b, c.IdentityDevLogin, a.devLogin),
		bus.Handle(b, c.IdentityGet, a.get),
		bus.Handle(b, c.IdentitySettingsGet, a.settingsGet),
		bus.Handle(b, c.IdentitySettingsPut, a.settingsPut),
	} {
		if err != nil {
			s.Fatal("subscribe", err)
		}
	}
	s.Ready()
	s.Wait()
}

type app struct{ db *pgxpool.Pool }

const accountCols = `id, COALESCE(telegram_id,0), COALESCE(username,''), display_name, COALESCE(language,''), created_at`

func scanAccount(row pgx.Row) (c.Account, error) {
	var a c.Account
	err := row.Scan(&a.ID, &a.TelegramID, &a.Username, &a.DisplayName, &a.Language, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, apperr.New(apperr.NotFound, "account not found")
	}
	return a, err
}

func (a *app) telegramLogin(ctx context.Context, req c.TelegramLoginReq) (c.Account, error) {
	u := req.User
	if u.ID == 0 {
		return c.Account{}, apperr.New(apperr.Invalid, "telegram user id required")
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = u.Username
	}
	return scanAccount(a.db.QueryRow(ctx, `
		INSERT INTO accounts (telegram_id, username, display_name, language)
		VALUES ($1, NULLIF($2,''), $3, NULLIF($4,''))
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username, display_name = EXCLUDED.display_name,
			language = EXCLUDED.language, last_login_at = now()
		RETURNING `+accountCols, u.ID, u.Username, name, u.LanguageCode))
}

var devNameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,24}$`)

func (a *app) devLogin(ctx context.Context, req c.DevLoginReq) (c.Account, error) {
	if !devNameRe.MatchString(req.Username) {
		return c.Account{}, apperr.New(apperr.Invalid, "username must be 3-24 letters, digits or _")
	}
	return scanAccount(a.db.QueryRow(ctx, `
		INSERT INTO accounts (dev_username, username, display_name)
		VALUES ($1, $1, $1)
		ON CONFLICT (dev_username) DO UPDATE SET last_login_at = now()
		RETURNING `+accountCols, req.Username))
}

func (a *app) get(ctx context.Context, req c.AccountReq) (c.Account, error) {
	return scanAccount(a.db.QueryRow(ctx, `SELECT `+accountCols+` FROM accounts WHERE id = $1`, req.AccountID))
}

// Only known keys may be stored, each with a size limit.
var settingKeys = map[string]int{"hud": 16 << 10, "prefs": 8 << 10}

func (a *app) settingsGet(ctx context.Context, req c.SettingsGetReq) (c.SettingsValue, error) {
	if _, ok := settingKeys[req.Key]; !ok {
		return c.SettingsValue{}, apperr.New(apperr.Invalid, "unknown setting %q", req.Key)
	}
	out := c.SettingsValue{Key: req.Key}
	err := a.db.QueryRow(ctx, `SELECT value FROM account_settings WHERE account_id=$1 AND key=$2`, req.AccountID, req.Key).Scan(&out.Value)
	if errors.Is(err, pgx.ErrNoRows) {
		out.Value = json.RawMessage("null")
		return out, nil
	}
	return out, err
}

func (a *app) settingsPut(ctx context.Context, req c.SettingsPutReq) (c.SettingsValue, error) {
	limit, ok := settingKeys[req.Key]
	if !ok {
		return c.SettingsValue{}, apperr.New(apperr.Invalid, "unknown setting %q", req.Key)
	}
	if len(req.Value) == 0 || len(req.Value) > limit || !json.Valid(req.Value) {
		return c.SettingsValue{}, apperr.New(apperr.Invalid, "setting value must be JSON up to %d bytes", limit)
	}
	_, err := a.db.Exec(ctx, `
		INSERT INTO account_settings (account_id, key, value) VALUES ($1,$2,$3)
		ON CONFLICT (account_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		req.AccountID, req.Key, []byte(req.Value))
	return c.SettingsValue{Key: req.Key, Value: req.Value}, err
}
