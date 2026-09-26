package main

import (
	"context"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/zone"
)

// interact uses whatever object stands on tile (x,y). Range and state are
// validated by the owning service; here we only decide who owns it.
func (a *app) interact(ctx context.Context, char string, x, y int) (any, error) {
	pos, err := bus.Request[c.Position](ctx, a.bus, c.PresenceGet, c.CharacterReq{CharacterID: char})
	if err != nil {
		return nil, err
	}
	z, err := zone.Parse(pos.Zone)
	if err != nil {
		return nil, err
	}
	_, obj, err := a.zones.Tile(ctx, z, x, y)
	if err != nil {
		return nil, err
	}
	switch obj {
	case world.ODungeonEntrance:
		cx, cy := world.ChunkOf(x, y)
		w, err := a.zones.World(ctx, z.World)
		if err != nil {
			return nil, err
		}
		e, ok := w.EntranceIn(cx, cy)
		if !ok {
			return nil, apperr.New(apperr.NotFound, "no entrance")
		}
		v, err := bus.Request[c.DungeonView](ctx, a.bus, c.DungeonEnter, c.DungeonEnterReq{CharacterID: char, EntranceID: e.ID})
		return map[string]any{"action": "dungeon_enter", "dungeon": v}, err
	case world.OStairsDown:
		v, err := bus.Request[c.DungeonView](ctx, a.bus, c.DungeonDescend, c.DungeonReq{CharacterID: char})
		return map[string]any{"action": "dungeon_descend", "dungeon": v}, err
	case world.ODungeonExit:
		v, err := bus.Request[c.LeaveResp](ctx, a.bus, c.DungeonLeave, c.DungeonReq{CharacterID: char})
		return map[string]any{"action": "dungeon_leave", "position": v.Position}, err
	case world.OChest:
		f, _, err := a.zones.Floor(ctx, z)
		if err != nil {
			return nil, err
		}
		for _, ch := range f.Chests {
			if ch.X == x && ch.Y == y {
				v, err := bus.Request[c.ChestResp](ctx, a.bus, c.DungeonOpenChest, c.DungeonReq{CharacterID: char, ChestID: ch.ID})
				return map[string]any{"action": "chest", "loot": v.Loot}, err
			}
		}
		return nil, apperr.New(apperr.NotFound, "no chest here")
	case world.OAnvil:
		return map[string]any{"action": "open_forge"}, nil
	case world.OShrine:
		return map[string]any{"action": "shrine", "text": "The shrine hums. This is where you return when you fall."}, nil
	}
	return nil, apperr.New(apperr.NotFound, "nothing to interact with")
}
