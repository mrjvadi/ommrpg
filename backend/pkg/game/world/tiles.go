package world

// Ground is the base terrain id of a tile. The numeric values are part of the
// wire protocol: the Godot client and the sprite-service tileset atlas index
// tiles by these ids, so only ever append new values.
type Ground uint8

const (
	GDeepWater Ground = iota
	GWater
	GSand
	GGrass
	GForest
	GDirt
	GSnow
	GSwamp
	GRock
	GAsh
	GPath
	GDungeonFloor
	GDungeonWall
	GIce
	GLava
	GShallow // river water: walkable
	GroundCount
)

var groundNames = [...]string{"deep_water", "water", "sand", "grass", "forest", "dirt", "snow", "swamp", "rock", "ash", "path", "dungeon_floor", "dungeon_wall", "ice", "lava", "shallow_water"}

func (g Ground) String() string {
	if int(g) < len(groundNames) {
		return groundNames[g]
	}
	return "unknown"
}

// Walkable reports whether a character can stand on this ground.
func (g Ground) Walkable() bool {
	switch g {
	case GDeepWater, GWater, GDungeonWall, GLava:
		return false
	}
	return true
}

// Object is the id of whatever stands on top of the ground (trees, rocks,
// dungeon entrances, chests...). 0 means empty. Wire protocol: append only.
type Object uint8

const (
	ONone Object = iota
	OTree
	OPine
	ORock
	OBush
	OFlowers
	OCactus
	ODeadTree
	OBoulder
	OReeds
	ODungeonEntrance
	OStairsDown
	ODungeonExit
	OChest
	OTorch
	OAnvil
	OShrine
	OBones
	ObjectCount
)

var objectNames = [...]string{"none", "tree", "pine", "rock", "bush", "flowers", "cactus", "dead_tree", "boulder", "reeds", "dungeon_entrance", "stairs_down", "dungeon_exit", "chest", "torch", "anvil", "shrine", "bones"}

func (o Object) String() string {
	if int(o) < len(objectNames) {
		return objectNames[o]
	}
	return "unknown"
}

// Blocking reports whether the object prevents walking through its tile.
func (o Object) Blocking() bool {
	switch o {
	case OTree, OPine, ORock, OCactus, ODeadTree, OBoulder, OChest, OTorch, OAnvil, OShrine:
		return true
	}
	return false
}

// Interactive objects are used through the "interact" action.
func (o Object) Interactive() bool {
	switch o {
	case ODungeonEntrance, OStairsDown, ODungeonExit, OChest, OAnvil, OShrine:
		return true
	}
	return false
}

// Biome classifies overworld regions; it drives ground, props and which
// species spawn. Dungeon is a pseudo-biome used by dungeon floors.
type Biome uint8

const (
	BOcean Biome = iota
	BBeach
	BPlains
	BForest
	BDesert
	BTundra
	BSwamp
	BMountain
	BTaiga
	BVolcanic
	BDungeon
	BiomeCount
)

var biomeNames = [...]string{"ocean", "beach", "plains", "forest", "desert", "tundra", "swamp", "mountain", "taiga", "volcanic", "dungeon"}

func (b Biome) String() string {
	if int(b) < len(biomeNames) {
		return biomeNames[b]
	}
	return "unknown"
}

// Legend describes ids for clients and tools (served by world-service).
func Legend() map[string]any {
	g := map[int]string{}
	for i := Ground(0); i < GroundCount; i++ {
		g[int(i)] = i.String()
	}
	o := map[int]string{}
	for i := Object(0); i < ObjectCount; i++ {
		o[int(i)] = i.String()
	}
	b := map[int]string{}
	for i := Biome(0); i < BiomeCount; i++ {
		b[int(i)] = i.String()
	}
	return map[string]any{"ground": g, "objects": o, "biomes": b, "chunk_size": ChunkSize}
}
