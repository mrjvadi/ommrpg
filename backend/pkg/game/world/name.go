package world

import (
	"github.com/mrjvadi/ommrpg/backend/pkg/game/names"
	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

func nameWord(r *seed.Rand) string { return names.Word(r, r.Range(2, 3)) }

// WorldName returns the generated display name of a world.
func WorldName(s uint64) string { return names.Name(seed.Derive(s, "world-name")) }
