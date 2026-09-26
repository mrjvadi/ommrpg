package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/centrifugo"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/items"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/progression"
)

type app struct {
	db     *pgxpool.Pool
	bus    *bus.Bus
	cf     *centrifugo.Client
	log    *slog.Logger
	secret uint64
	maxInv int
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type stored struct {
	ID        string
	Owner     string
	Item      items.Item
	State     items.State
	Attempts  int64
	Equipped  string
	Source    string
	CreatedAt time.Time
}

const itemCols = `id, owner_id, item, enhance, level, xp, enhance_attempts, COALESCE(equipped_slot,''), source, created_at`

func scanItem(r pgx.Row) (*stored, error) {
	var s stored
	var raw []byte
	err := r.Scan(&s.ID, &s.Owner, &raw, &s.State.Enhance, &s.State.Level, &s.State.XP, &s.Attempts, &s.Equipped, &s.Source, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.New(apperr.NotFound, "item not found")
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.Item); err != nil {
		return nil, err
	}
	return &s, nil
}

func view(s *stored) c.OwnedItem {
	o := c.OwnedItem{
		ID: s.ID, Item: s.Item, State: s.State, Equipped: s.Equipped, Source: s.Source, CreatedAt: s.CreatedAt,
		DisplayName: items.DisplayName(s.Item, s.State),
		Stats:       items.Effective(s.Item, s.State),
	}
	if s.State.Level < s.Item.Rarity.MaxLevel() {
		o.XPToNext = items.XPToNext(s.State.Level)
	}
	if s.State.Enhance < s.Item.Rarity.MaxEnhance() {
		o.EnhanceChance = items.EnhanceChance(s.State.Enhance, 0)
		o.EnhanceGold, o.EnhanceEssence = items.EnhanceCost(s.Item, s.State.Enhance)
	}
	o.SalvageEssence, o.SalvageGold = items.SalvageValue(s.Item, s.State)
	return o
}

func (a *app) loadItem(ctx context.Context, q querier, id, owner string, lock bool) (*stored, error) {
	sql := `SELECT ` + itemCols + ` FROM items WHERE id=$1 AND owner_id=$2`
	if lock {
		sql += ` FOR UPDATE`
	}
	return scanItem(q.QueryRow(ctx, sql, id, owner))
}

func wallet(ctx context.Context, q querier, owner string, lock bool) (c.Wallet, error) {
	sql := `SELECT gold, essence FROM wallets WHERE owner_id=$1`
	if lock {
		sql += ` FOR UPDATE`
	}
	var w c.Wallet
	err := q.QueryRow(ctx, sql, owner).Scan(&w.Gold, &w.Essence)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.Wallet{}, nil
	}
	return w, err
}

// credit changes balances and writes the ledger. Negative deltas must be
// pre-checked; the CHECK constraints are the last line of defence.
func credit(ctx context.Context, tx pgx.Tx, owner string, gold, essence int64, reason, ref string) (c.Wallet, error) {
	var w c.Wallet
	// Ensure the row first: an upsert would evaluate the CHECK constraints on
	// the (negative) insert tuple before falling back to the update.
	if _, err := tx.Exec(ctx, `INSERT INTO wallets (owner_id) VALUES ($1) ON CONFLICT DO NOTHING`, owner); err != nil {
		return w, err
	}
	err := tx.QueryRow(ctx, `UPDATE wallets SET gold = gold + $2, essence = essence + $3 WHERE owner_id = $1
		RETURNING gold, essence`, owner, gold, essence).Scan(&w.Gold, &w.Essence)
	if err != nil {
		return w, err
	}
	for _, e := range []struct {
		cur   string
		delta int64
		bal   int64
	}{{"gold", gold, w.Gold}, {"essence", essence, w.Essence}} {
		if e.delta == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO ledger (owner_id, currency, delta, balance_after, reason, ref) VALUES ($1,$2,$3,$4,$5,$6)`,
			owner, e.cur, e.delta, e.bal, reason, ref); err != nil {
			return w, err
		}
	}
	return w, nil
}

func (a *app) list(ctx context.Context, req c.ItemActionReq) (c.InventoryResp, error) {
	rows, err := a.db.Query(ctx, `SELECT `+itemCols+` FROM items WHERE owner_id=$1 ORDER BY equipped_slot NULLS LAST, created_at DESC`, req.CharacterID)
	if err != nil {
		return c.InventoryResp{}, err
	}
	defer rows.Close()
	out := c.InventoryResp{Items: []c.OwnedItem{}}
	for rows.Next() {
		s, err := scanItem(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, view(s))
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	out.Wallet, err = wallet(ctx, a.db, req.CharacterID, false)
	return out, err
}

func (a *app) equip(ctx context.Context, req c.ItemActionReq) (c.InventoryResp, error) {
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		it, err := a.loadItem(ctx, tx, req.ItemID, req.CharacterID, true)
		if err != nil {
			return err
		}
		slot := string(it.Item.Slot)
		if _, err := tx.Exec(ctx, `UPDATE items SET equipped_slot=NULL WHERE owner_id=$1 AND equipped_slot=$2`, req.CharacterID, slot); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE items SET equipped_slot=$3 WHERE id=$1 AND owner_id=$2`, req.ItemID, req.CharacterID, slot)
		return err
	})
	if err != nil {
		return c.InventoryResp{}, err
	}
	return a.list(ctx, req)
}

func (a *app) unequip(ctx context.Context, req c.ItemActionReq) (c.InventoryResp, error) {
	if !items.ValidSlot(req.Slot) {
		return c.InventoryResp{}, apperr.New(apperr.Invalid, "unknown slot %q", req.Slot)
	}
	if _, err := a.db.Exec(ctx, `UPDATE items SET equipped_slot=NULL WHERE owner_id=$1 AND equipped_slot=$2`, req.CharacterID, req.Slot); err != nil {
		return c.InventoryResp{}, err
	}
	return a.list(ctx, req)
}

// idempotent returns a stored response for a repeated request id.
func idempotent[T any](ctx context.Context, tx pgx.Tx, reqID, owner string) (T, bool, error) {
	var zero T
	if reqID == "" {
		return zero, false, apperr.New(apperr.Invalid, "request_id is required")
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT response FROM idempotency WHERE request_id=$1 AND owner_id=$2`, reqID, owner).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	err = json.Unmarshal(raw, &zero)
	return zero, true, err
}

func remember(ctx context.Context, tx pgx.Tx, reqID, owner string, resp any) error {
	raw, _ := json.Marshal(resp)
	_, err := tx.Exec(ctx, `INSERT INTO idempotency (request_id, owner_id, response) VALUES ($1,$2,$3)`, reqID, owner, raw)
	return err
}

func (a *app) enhance(ctx context.Context, req c.ItemActionReq) (c.EnhanceResp, error) {
	prof, err := bus.Request[c.CombatProfile](ctx, a.bus, c.CharacterCombatProfile, c.CharacterReq{CharacterID: req.CharacterID})
	if err != nil {
		return c.EnhanceResp{}, err
	}
	var out c.EnhanceResp
	var replay bool
	err = pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		// serialise per owner so idempotency + balance checks are race-free
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "wallet:"+req.CharacterID); err != nil {
			return err
		}
		prev, ok, err := idempotent[c.EnhanceResp](ctx, tx, req.RequestID, req.CharacterID)
		if err != nil || ok {
			out, replay = prev, ok
			return err
		}
		it, err := a.loadItem(ctx, tx, req.ItemID, req.CharacterID, true)
		if err != nil {
			return err
		}
		if it.State.Enhance >= it.Item.Rarity.MaxEnhance() {
			return apperr.New(apperr.Conflict, "already at max enhancement +%d", it.State.Enhance)
		}
		gold, ess := items.EnhanceCost(it.Item, it.State.Enhance)
		w, err := wallet(ctx, tx, req.CharacterID, true)
		if err != nil {
			return err
		}
		if w.Gold < gold || w.Essence < ess {
			return apperr.New(apperr.InsufficientFunds, "need %d gold and %d essence", gold, ess)
		}
		attempt := it.Attempts + 1
		res, err := items.Enhance(a.secret, it.ID, attempt, it.Item, it.State, prof.Hidden.UpgradeBonus())
		if err != nil {
			return apperr.New(apperr.Conflict, "%v", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE items SET enhance=$2, enhance_attempts=$3 WHERE id=$1`, it.ID, res.To, attempt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO enhance_log (item_id, owner_id, attempt, from_level, to_level, chance, roll, gold, essence)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, it.ID, req.CharacterID, attempt, res.From, res.To, res.Chance, res.Roll, gold, ess); err != nil {
			return err
		}
		w, err = credit(ctx, tx, req.CharacterID, -gold, -ess, "enhance", it.ID)
		if err != nil {
			return err
		}
		it.State.Enhance = res.To
		out = c.EnhanceResp{Result: res, Item: view(it), Wallet: w}
		return remember(ctx, tx, req.RequestID, req.CharacterID, out)
	})
	if err != nil {
		return c.EnhanceResp{}, err
	}
	if !replay {
		_ = a.bus.Publish(ctx, c.EvItemEnhanced, "enhanced:"+req.RequestID, c.ItemEnhancedEv{
			CharacterID: req.CharacterID, ItemID: out.Item.ID, ItemName: out.Item.Item.Name, Result: out.Result})
	}
	return out, nil
}

func (a *app) salvage(ctx context.Context, req c.ItemActionReq) (c.SalvageResp, error) {
	var out c.SalvageResp
	var replay bool
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "wallet:"+req.CharacterID); err != nil {
			return err
		}
		prev, ok, err := idempotent[c.SalvageResp](ctx, tx, req.RequestID, req.CharacterID)
		if err != nil || ok {
			out, replay = prev, ok
			return err
		}
		it, err := a.loadItem(ctx, tx, req.ItemID, req.CharacterID, true)
		if err != nil {
			return err
		}
		if it.Equipped != "" {
			return apperr.New(apperr.Conflict, "unequip the item before salvaging it")
		}
		ess, gold := items.SalvageValue(it.Item, it.State)
		if _, err := tx.Exec(ctx, `DELETE FROM items WHERE id=$1`, it.ID); err != nil {
			return err
		}
		w, err := credit(ctx, tx, req.CharacterID, gold, ess, "salvage", it.ID)
		if err != nil {
			return err
		}
		out = c.SalvageResp{Essence: ess, Gold: gold, Wallet: w}
		return remember(ctx, tx, req.RequestID, req.CharacterID, out)
	})
	if err != nil {
		return out, err
	}
	if !replay {
		_ = a.bus.Publish(ctx, c.EvItemSalvaged, "salvaged:"+req.ItemID, c.ItemSalvagedEv{CharacterID: req.CharacterID, ItemID: req.ItemID})
	}
	return out, nil
}

func (a *app) bonuses(ctx context.Context, req c.ItemActionReq) (c.BonusesResp, error) {
	rows, err := a.db.Query(ctx, `SELECT `+itemCols+` FROM items WHERE owner_id=$1 AND equipped_slot IS NOT NULL ORDER BY equipped_slot`, req.CharacterID)
	if err != nil {
		return c.BonusesResp{}, err
	}
	defer rows.Close()
	var out c.BonusesResp
	b := &out.Bonuses
	for rows.Next() {
		s, err := scanItem(rows)
		if err != nil {
			return out, err
		}
		for stat, v := range items.Effective(s.Item, s.State) {
			addStat(b, stat, v)
		}
		if s.Item.Slot == items.Weapon {
			b.WeaponKind, b.Range, b.Cooldown = s.Item.Kind, s.Item.Range, s.Item.Cooldown
		}
		if ap := s.Item.Appearance; ap != nil {
			out.Looks = append(out.Looks, c.Layer{Item: ap.Item, Variant: ap.Variant, Colors: ap.Colors})
		}
	}
	return out, rows.Err()
}

func addStat(b *progression.Bonuses, stat string, v float64) {
	switch stat {
	case items.StatStr:
		b.Str += int(v)
	case items.StatAgi:
		b.Agi += int(v)
	case items.StatInt:
		b.Int += int(v)
	case items.StatVit:
		b.Vit += int(v)
	case items.StatDamage:
		b.Damage += int(v)
	case items.StatMagic:
		b.Magic += int(v)
	case items.StatArmor:
		b.Armor += int(v)
	case items.StatHP:
		b.HP += int(v)
	case items.StatCrit:
		b.CritPct += v
	case items.StatSpeed:
		b.SpeedPct += v
	case items.StatLifeSteal:
		b.LifeSteal += v
	case items.StatXP:
		b.XPPct += v
	case items.StatMagicFind:
		b.MagicFind += v
	}
}

// ---------------------------------------------------------------- events

type granted struct {
	ID   string     `json:"id"`
	Item items.Item `json:"item"`
}

func (a *app) grant(ctx context.Context, tx pgx.Tx, owner string, it items.Item, source, ref string, equip bool) (*granted, error) {
	raw, _ := json.Marshal(it)
	var id string
	var slot any
	if equip {
		slot = string(it.Slot)
	}
	err := tx.QueryRow(ctx, `INSERT INTO items (owner_id, seed, item, rarity, slot, source, source_ref, equipped_slot)
		VALUES ($1, $2::numeric, $3, $4, $5, $6, $7, $8) ON CONFLICT (source_ref) DO NOTHING RETURNING id`,
		owner, strconv.FormatUint(it.Seed, 10), raw, it.Rarity.String(), string(it.Slot), source, ref, slot).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // already granted
	}
	if err != nil {
		return nil, err
	}
	return &granted{ID: id, Item: it}, nil
}

func (a *app) onEvent(ctx context.Context, ev bus.Event) error {
	var owner string
	var loot items.Loot
	var source string
	var itemXP int64
	var starter bool
	switch ev.Type {
	case c.EvCharacterCreated:
		d, err := bus.Decode[c.CharacterCreatedEv](ev)
		if err != nil {
			return nil
		}
		owner, starter, source = d.CharacterID, true, "starter"
		loot = items.Loot{Gold: 120, Essence: 6}
	case c.EvMonsterKilled:
		d, err := bus.Decode[c.MonsterKilledEv](ev)
		if err != nil {
			return nil
		}
		owner, loot, source, itemXP = d.CharacterID, d.Loot, "kill:"+d.SpeciesName, d.XP/2
	case c.EvChestOpened:
		d, err := bus.Decode[c.ChestOpenedEv](ev)
		if err != nil {
			return nil
		}
		owner, loot, source = d.CharacterID, d.Loot, "chest"
	default:
		return nil
	}
	var drops []*granted
	var leveled []c.ItemLeveledEv
	var salvaged int64
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`, ev.ID)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		if starter {
			for i, base := range []string{"sword", "leather"} {
				it, _ := items.GenerateBase(seedOf(owner, i), base, 1, items.Common)
				g, err := a.grant(ctx, tx, owner, it, "starter", fmt.Sprintf("starter:%s:%d", owner, i), true)
				if err != nil {
					return err
				}
				if g != nil {
					drops = append(drops, g)
				}
			}
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM items WHERE owner_id=$1`, owner).Scan(&count); err != nil {
			return err
		}
		for i, d := range loot.Items {
			it := items.Generate(d.Seed, d.ItemLevel, d.Luck, d.MinRarity)
			if count >= a.maxInv {
				// full bag: the drop is salvaged automatically
				ess, _ := items.SalvageValue(it, items.State{Level: 1})
				salvaged += ess
				continue
			}
			g, err := a.grant(ctx, tx, owner, it, source, fmt.Sprintf("%s:%d", ev.ID, i), false)
			if err != nil {
				return err
			}
			if g != nil {
				drops = append(drops, g)
				count++
			}
		}
		if loot.Gold != 0 || loot.Essence != 0 || salvaged != 0 {
			if _, err := credit(ctx, tx, owner, loot.Gold, loot.Essence+salvaged, source, ev.ID); err != nil {
				return err
			}
		}
		if itemXP > 0 {
			rows, err := tx.Query(ctx, `SELECT `+itemCols+` FROM items WHERE owner_id=$1 AND equipped_slot IS NOT NULL FOR UPDATE`, owner)
			if err != nil {
				return err
			}
			var eq []*stored
			for rows.Next() {
				s, err := scanItem(rows)
				if err != nil {
					rows.Close()
					return err
				}
				eq = append(eq, s)
			}
			rows.Close()
			for _, s := range eq {
				if items.AddXP(s.Item, &s.State, itemXP) > 0 {
					leveled = append(leveled, c.ItemLeveledEv{CharacterID: owner, ItemID: s.ID, ItemName: s.Item.Name, Level: s.State.Level})
				}
				if _, err := tx.Exec(ctx, `UPDATE items SET level=$2, xp=$3 WHERE id=$1`, s.ID, s.State.Level, s.State.XP); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, d := range drops {
		_ = a.bus.Publish(ctx, c.EvItemDropped, "dropped:"+d.ID, c.ItemDroppedEv{CharacterID: owner, ItemID: d.ID, Item: d.Item, Source: source})
	}
	for _, l := range leveled {
		_ = a.bus.Publish(ctx, c.EvItemLeveled, fmt.Sprintf("item-leveled:%s:%d", l.ItemID, l.Level), l)
	}
	if len(drops) > 0 || loot.Gold > 0 || len(leveled) > 0 {
		names := []map[string]any{}
		for _, d := range drops {
			names = append(names, map[string]any{"id": d.ID, "name": d.Item.Name, "rarity": d.Item.Rarity.String(), "icon": d.Item.Icon, "icon_seed": strconv.FormatUint(d.Item.IconSeed, 10), "hue": d.Item.Hue})
		}
		_ = a.cf.Publish(ctx, centrifugo.PersonalChannel(owner), map[string]any{
			"t": "loot", "gold": loot.Gold, "essence": loot.Essence + salvaged, "items": names, "item_levels": leveled, "auto_salvaged": salvaged > 0,
		})
	}
	return nil
}

func seedOf(owner string, i int) uint64 {
	var h uint64 = 1469598103934665603
	for _, b := range []byte(owner) {
		h = (h ^ uint64(b)) * 1099511628211
	}
	return h + uint64(i)*0x9e3779b97f4a7c15
}
