package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/progression"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/zone"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

const maxCharactersPerAccount = 4

type app struct {
	db    *pgxpool.Pool
	bus   *bus.Bus
	cf    *centrifugo.Client
	log   *slog.Logger
	looks *lru.Cache[string, c.BonusesResp]
}

func newApp(db *pgxpool.Pool, b *bus.Bus, cf *centrifugo.Client, log *slog.Logger) *app {
	return &app{db: db, bus: b, cf: cf, log: log, looks: lru.New[string, c.BonusesResp](4096, 4*time.Second)}
}

// row is the full stored character.
type row struct {
	c.Character
	Hidden    progression.Hidden
	Behaviour progression.Behaviour
	Zone      string
	X, Y      float64
}

const cols = `id, account_id, name, world_id, level, xp, attr_str, attr_agi, attr_int, attr_vit,
	free_points, class, hidden, behaviour, appearance, zone, pos_x, pos_y, created_at`

func scan(r pgx.Row) (*row, error) {
	var x row
	var hidden, beh, app []byte
	err := r.Scan(&x.ID, &x.AccountID, &x.Name, &x.WorldID, &x.Level, &x.XP,
		&x.Attributes.Str, &x.Attributes.Agi, &x.Attributes.Int, &x.Attributes.Vit,
		&x.FreePoints, &x.Class, &hidden, &beh, &app, &x.Zone, &x.X, &x.Y, &x.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.New(apperr.NotFound, "character not found")
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(hidden, &x.Hidden)
	_ = json.Unmarshal(beh, &x.Behaviour)
	_ = json.Unmarshal(app, &x.Appearance)
	x.XPToNext = progression.XPToNext(x.Level)
	x.RootHint = rootHint(x.Hidden.Root)
	return &x, nil
}

func (a *app) load(ctx context.Context, q pgx.Tx, id string, forUpdate bool) (*row, error) {
	sql := `SELECT ` + cols + ` FROM characters WHERE id = $1`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	if q != nil {
		return scan(q.QueryRow(ctx, sql, id))
	}
	return scan(a.db.QueryRow(ctx, sql, id))
}

// The Root is hidden; players only ever see a vague omen.
func rootHint(r progression.Root) string {
	switch r {
	case progression.Starforged:
		return "Your blood hums like distant stars."
	case progression.Voidborn:
		return "Shadows lean toward you when you pass."
	case progression.SoulWeaver:
		return "You sometimes hear the dead whisper."
	case progression.SpiritVessel:
		return "Wild spirits find you strangely calm."
	case progression.BeastHeart:
		return "Animals meet your gaze without fear."
	}
	return ""
}

var nameRe = regexp.MustCompile(`^[\p{L}\p{N}_ ]+$`)

func validName(n string) (string, error) {
	n = strings.Join(strings.Fields(n), " ")
	l := utf8.RuneCountInString(n)
	if l < 3 || l > 16 || !nameRe.MatchString(n) {
		return "", apperr.New(apperr.Invalid, "name must be 3-16 letters, digits, spaces or _")
	}
	return n, nil
}

func (a *app) create(ctx context.Context, req c.CreateCharacterReq) (c.Character, error) {
	name, err := validName(req.Name)
	if err != nil {
		return c.Character{}, err
	}
	var n int
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM characters WHERE account_id=$1`, req.AccountID).Scan(&n); err != nil {
		return c.Character{}, err
	}
	if n >= maxCharactersPerAccount {
		return c.Character{}, apperr.New(apperr.Conflict, "you already have %d characters", maxCharactersPerAccount)
	}
	w, err := bus.Request[c.WorldInfo](ctx, a.bus, c.WorldDefault, struct{}{})
	if err != nil {
		return c.Character{}, err
	}
	var id string
	if err := a.db.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
		return c.Character{}, err
	}
	s := seed.FromString(id)
	var recipe c.Recipe
	if req.Appearance != nil {
		v, err := bus.Request[c.SpriteValidateResp](ctx, a.bus, c.SpriteValidate, req.Appearance)
		if err != nil {
			return c.Character{}, err
		}
		recipe = v.Recipe
	} else {
		if recipe, err = bus.Request[c.Recipe](ctx, a.bus, c.SpriteRandom, c.SpriteRandomReq{Seed: s, BodyType: req.BodyType}); err != nil {
			return c.Character{}, err
		}
	}
	hidden := progression.RollHidden(s)
	hj, _ := json.Marshal(hidden)
	rj, _ := json.Marshal(recipe)
	_, err = a.db.Exec(ctx, `INSERT INTO characters (id, account_id, name, world_id, hidden, appearance, zone, pos_x, pos_y)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, req.AccountID, name, w.ID, hj, rj, zone.World(w.ID).String(), float64(w.SpawnX)+0.5, float64(w.SpawnY)+1.5)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return c.Character{}, apperr.New(apperr.Conflict, "the name %q is taken", name)
	}
	if err != nil {
		return c.Character{}, err
	}
	if err := a.bus.Publish(ctx, c.EvCharacterCreated, "character-created:"+id,
		c.CharacterCreatedEv{CharacterID: id, AccountID: req.AccountID, Name: name, WorldID: w.ID}); err != nil {
		a.log.Error("publish character.created", "err", err)
	}
	r, err := a.load(ctx, nil, id, false)
	if err != nil {
		return c.Character{}, err
	}
	return r.Character, nil
}

func (a *app) list(ctx context.Context, req c.AccountReq) (c.CharacterListResp, error) {
	rows, err := a.db.Query(ctx, `SELECT `+cols+` FROM characters WHERE account_id=$1 ORDER BY created_at`, req.AccountID)
	if err != nil {
		return c.CharacterListResp{}, err
	}
	defer rows.Close()
	out := c.CharacterListResp{Characters: []c.Character{}}
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			return out, err
		}
		out.Characters = append(out.Characters, r.Character)
	}
	return out, rows.Err()
}

func (a *app) equipment(ctx context.Context, id string) c.BonusesResp {
	if v, ok := a.looks.Get(id); ok {
		return v
	}
	v, err := bus.Request[c.BonusesResp](ctx, a.bus, c.ItemBonuses, c.ItemActionReq{CharacterID: id})
	if err != nil {
		a.log.Warn("item bonuses unavailable", "err", err)
		return c.BonusesResp{}
	}
	a.looks.Put(id, v)
	return v
}

// dressed merges equipment looks over the base appearance.
func dressed(base c.Recipe, eq c.BonusesResp) c.Recipe {
	out := c.Recipe{BodyType: base.BodyType, Layers: append([]c.Layer(nil), base.Layers...)}
	out.Layers = append(out.Layers, eq.Looks...)
	return out
}

func (a *app) get(ctx context.Context, req c.CharacterReq) (c.Character, error) {
	r, err := a.load(ctx, nil, req.CharacterID, false)
	if err != nil {
		return c.Character{}, err
	}
	if req.AccountID != "" && r.AccountID != req.AccountID {
		return c.Character{}, apperr.New(apperr.Forbidden, "not your character")
	}
	eq := a.equipment(ctx, r.ID)
	d := progression.Derive(r.Level, r.Attributes, r.Hidden, eq.Bonuses)
	r.Derived = &d
	r.Appearance = dressed(r.Appearance, eq)
	r.FX = eq.FX
	return r.Character, nil
}

func (a *app) public(ctx context.Context, req c.CharacterReq) (c.PublicCharacter, error) {
	r, err := a.load(ctx, nil, req.CharacterID, false)
	if err != nil {
		return c.PublicCharacter{}, err
	}
	eq := a.equipment(ctx, r.ID)
	return c.PublicCharacter{ID: r.ID, Name: r.Name, Level: r.Level, Class: r.Class, Appearance: dressed(r.Appearance, eq), FX: eq.FX}, nil
}

func (a *app) allocate(ctx context.Context, req c.AllocateReq) (c.Character, error) {
	p := req.Points
	if !p.Valid() || p.Sum() == 0 {
		return c.Character{}, apperr.New(apperr.Invalid, "allocate a positive number of points")
	}
	tag, err := a.db.Exec(ctx, `UPDATE characters SET attr_str=attr_str+$2, attr_agi=attr_agi+$3, attr_int=attr_int+$4,
		attr_vit=attr_vit+$5, free_points=free_points-$6, updated_at=now() WHERE id=$1 AND free_points >= $6`,
		req.CharacterID, p.Str, p.Agi, p.Int, p.Vit, p.Sum())
	if err != nil {
		return c.Character{}, err
	}
	if tag.RowsAffected() == 0 {
		return c.Character{}, apperr.New(apperr.Invalid, "not enough free points")
	}
	a.looks.Delete(req.CharacterID)
	return a.get(ctx, c.CharacterReq{CharacterID: req.CharacterID})
}

func (a *app) combatProfile(ctx context.Context, req c.CharacterReq) (c.CombatProfile, error) {
	r, err := a.load(ctx, nil, req.CharacterID, false)
	if err != nil {
		return c.CombatProfile{}, err
	}
	eq := a.equipment(ctx, r.ID)
	return c.CombatProfile{CharacterID: r.ID, Level: r.Level, Hidden: r.Hidden, WorldID: r.WorldID,
		Derived: progression.Derive(r.Level, r.Attributes, r.Hidden, eq.Bonuses)}, nil
}

func (a *app) locationGet(ctx context.Context, req c.CharacterReq) (c.Location, error) {
	r, err := a.load(ctx, nil, req.CharacterID, false)
	if err != nil {
		return c.Location{}, err
	}
	return c.Location{CharacterID: r.ID, Zone: r.Zone, X: r.X, Y: r.Y}, nil
}

func (a *app) locationSet(ctx context.Context, req c.Location) (struct{}, error) {
	if _, err := zone.Parse(req.Zone); err != nil {
		return struct{}{}, apperr.New(apperr.Invalid, "%v", err)
	}
	_, err := a.db.Exec(ctx, `UPDATE characters SET zone=$2, pos_x=$3, pos_y=$4, updated_at=now() WHERE id=$1`,
		req.CharacterID, req.Zone, req.X, req.Y)
	return struct{}{}, err
}

// ---------------------------------------------------------------- events

func (a *app) onEvent(ctx context.Context, ev bus.Event) error {
	var charID string
	var apply func(r *row)
	switch ev.Type {
	case c.EvMonsterKilled:
		d, err := bus.Decode[c.MonsterKilledEv](ev)
		if err != nil {
			return nil
		}
		charID = d.CharacterID
		apply = func(r *row) {
			switch d.WeaponKind {
			case "ranged":
				r.Behaviour.RangedKills++
			case "magic":
				r.Behaviour.MagicKills++
			default:
				r.Behaviour.MeleeKills++
			}
			r.XP += d.XP
		}
	case c.EvItemEnhanced:
		d, _ := bus.Decode[c.ItemEnhancedEv](ev)
		charID, apply = d.CharacterID, func(r *row) { r.Behaviour.Upgrades++ }
	case c.EvItemSalvaged:
		d, _ := bus.Decode[c.ItemSalvagedEv](ev)
		charID, apply = d.CharacterID, func(r *row) { r.Behaviour.Salvages++ }
	case c.EvDungeonCleared:
		d, _ := bus.Decode[c.DungeonClearedEv](ev)
		charID, apply = d.CharacterID, func(r *row) { r.Behaviour.DungeonClear++ }
	case c.EvCharacterDied:
		d, _ := bus.Decode[c.CharacterDiedEv](ev)
		charID, apply = d.CharacterID, func(r *row) { r.Behaviour.Deaths++ }
	default:
		return nil
	}
	if charID == "" {
		return nil
	}
	var leveled []int
	var awakened string
	var name string
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`, ev.ID)
		if err != nil || tag.RowsAffected() == 0 {
			return err // already processed
		}
		r, err := a.load(ctx, tx, charID, true)
		if apperr.Is(err, apperr.NotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		name = r.Name
		apply(r)
		for r.Level < progression.MaxLevel && r.XP >= progression.XPToNext(r.Level) {
			r.XP -= progression.XPToNext(r.Level)
			r.Level++
			r.FreePoints += progression.PointsPerLevel
			leveled = append(leveled, r.Level)
		}
		if r.Class == "" && r.Level >= progression.AwakeningLevel {
			r.Class = string(progression.Awaken(r.Attributes, r.Hidden, r.Behaviour))
			awakened = r.Class
		}
		bj, _ := json.Marshal(r.Behaviour)
		_, err = tx.Exec(ctx, `UPDATE characters SET level=$2, xp=$3, free_points=$4, class=$5, behaviour=$6, updated_at=now() WHERE id=$1`,
			r.ID, r.Level, r.XP, r.FreePoints, r.Class, bj)
		return err
	})
	if err != nil {
		return err
	}
	for _, l := range leveled {
		_ = a.bus.Publish(ctx, c.EvCharacterLeveled, fmt.Sprintf("leveled:%s:%d", charID, l), c.CharacterLeveledEv{CharacterID: charID, Name: name, Level: l})
		_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(charID), map[string]any{"t": "level_up", "level": l, "points": progression.PointsPerLevel})
	}
	if awakened != "" {
		_ = a.bus.Publish(ctx, c.EvCharacterAwakened, "awakened:"+charID, c.CharacterAwakenedEv{CharacterID: charID, Name: name, Class: awakened})
		_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(charID), map[string]any{"t": "awakened", "class": awakened})
	}
	if ev.Type == c.EvMonsterKilled {
		_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(charID), map[string]any{"t": "xp"})
	}
	return nil
}
