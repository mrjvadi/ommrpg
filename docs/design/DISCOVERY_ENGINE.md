# DISCOVERY_ENGINE.md

## Purpose
Generate new knowledge, techniques, recipes, items, classes, professions, formations,
creatures and other outcomes from world rules.

## Inputs
- knowledge
- concepts
- understanding
- mastery
- root
- hidden talent
- energies
- affinities
- materials
- equipment
- environment
- region
- world
- event
- sequence of operations
- player state
- controlled seed

## Output Types
- knowledge
- technique
- recipe
- item
- artifact
- weapon
- armor
- potion
- formation
- creature
- soldier
- class/path
- profession
- city capability

## Determinism
Use seeded randomness.
Same validated state + seed must produce a reproducible outcome.

## Concept Graph
Knowledge should expose conceptual tags.
Example:
Necromancy -> death, soul, corpse, decay, control.
Demonic knowledge -> contract, corruption, chaos, blood, fire.

Combinations use these concepts, not only item names.

## Environment
The same knowledge can produce different outcomes in different:
- worlds
- regions
- events
- energy states

## AI Usage
AI can provide:
- names
- flavor text
- lore
- narrative explanation

Game Engine owns:
- result
- prerequisites
- costs
- rarity
- ownership
- exact progression impact

## World-First
Track:
- first discoverer
- first creator
- timestamp
- world
- region
- discovery conditions
- current owner
- lineage

## Security
Protect unique discoveries against:
- duplicates
- replay
- fake prerequisites
- seed manipulation
- race conditions
