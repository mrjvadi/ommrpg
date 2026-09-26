// Package zone names the places a character can be: the overworld of a world
// ("w:<world>") or one floor of a dungeon instance ("d:<instance>:<floor>").
package zone

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
)

type Kind string

const (
	Overworld Kind = "w"
	Dungeon   Kind = "d"
)

type ID struct {
	Kind     Kind
	World    int
	Instance string
	Floor    int
}

func World(id int) ID { return ID{Kind: Overworld, World: id} }

func DungeonFloor(instance string, floor int) ID {
	return ID{Kind: Dungeon, Instance: instance, Floor: floor}
}

func (z ID) String() string {
	if z.Kind == Dungeon {
		return fmt.Sprintf("d:%s:%d", z.Instance, z.Floor)
	}
	return fmt.Sprintf("w:%d", z.World)
}

func Parse(s string) (ID, error) {
	parts := strings.Split(s, ":")
	switch {
	case len(parts) == 2 && parts[0] == "w":
		w, err := strconv.Atoi(parts[1])
		if err != nil {
			return ID{}, fmt.Errorf("bad world zone %q", s)
		}
		return World(w), nil
	case len(parts) == 3 && parts[0] == "d":
		f, err := strconv.Atoi(parts[2])
		if err != nil || parts[1] == "" {
			return ID{}, fmt.Errorf("bad dungeon zone %q", s)
		}
		return DungeonFloor(parts[1], f), nil
	}
	return ID{}, fmt.Errorf("bad zone %q", s)
}

// Area is the realtime broadcast cell a position belongs to. In the
// overworld it is the chunk; a dungeon floor is a single area.
func (z ID) Area(x, y float64) (int, int) {
	if z.Kind == Dungeon {
		return 0, 0
	}
	return world.ChunkOf(int(x), int(y))
}

// Channel returns the Centrifugo channel for an area of this zone.
func (z ID) Channel(ax, ay int) string {
	if z.Kind == Dungeon {
		return fmt.Sprintf("dungeon:%s_%d", z.Instance, z.Floor)
	}
	return fmt.Sprintf("area:w%d_%d_%d", z.World, ax, ay)
}

// NeighbourAreas lists the areas visible from an area (3x3 in overworld).
func (z ID) NeighbourAreas(ax, ay int) [][2]int {
	if z.Kind == Dungeon {
		return [][2]int{{0, 0}}
	}
	out := make([][2]int, 0, 9)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			out = append(out, [2]int{ax + dx, ay + dy})
		}
	}
	return out
}
