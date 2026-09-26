# WORLD_SYSTEM.md

## Multi-World Universe
The architecture must support many worlds/realms.

Example:
- Mortal Realm
- Spirit Realm
- Divine Realm
- Abyss Realm
- Eternal Realm

## World Access
Level ranges may exist but are not sufficient.
Ascension can require:
- level
- knowledge
- mastery
- understanding
- item
- achievement
- trial
- event

## World Separation
Prevent high-level farming of lower-world players.
Special arenas/events may allow exceptions.

## World Specialization
Different worlds have different:
- resources
- monsters
- materials
- events
- knowledge
- gates

## Cross-World Dependencies
Higher worlds may depend on low-world resources.
Low worlds therefore remain economically relevant.

## Legacy
When a player ascends:
- personal progression moves forward
- historical influence may remain
- schools and businesses may remain
- shared knowledge can remain
- city/Kingdom history records the character

## AI Population
Worlds contain persistent AI characters that act, trade and learn even when players are offline.

## World Events
Examples:
- eclipse
- volcano
- meteor
- ancient beast
- resource bloom
- dimensional anomaly
- world boss
- ancient ruins

## Seed-Only Persistence (implemented)

`world-service` persists only each world's `seed` and static structure (size,
name). Terrain, biomes, props, dungeon entrances, monster species and monster
spawns are pure, deterministic functions of the seed, recomputed on demand by
any service through the shared `backend/pkg/game` packages (built on the single
splitmix64 primitive in `backend/pkg/seed`). See `docs/PROCEDURAL_GENERATION.md`.

## Starter Plaza and Danger Gradient (implemented)

Every world has a paved starter plaza with a shrine (respawn point) and an
anvil (upgrades) at the walkable spot closest to the world centre. A safe
radius around it has no monsters or dungeon entrances. Danger (monster level)
grows with distance from the plaza, so exploring outward is the progression
path. Dungeon entrances are placed by the world seed; each entrance has a fixed
layout seed and each run is a fresh instance.
