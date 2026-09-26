# Roadmap

Status of the design phases from `docs/design` in this codebase.

## Done (this milestone)

* **Foundation**: Go microservices, NATS request/reply + JetStream events,
  PostgreSQL per service with embedded migrations, Dragonfly hot state,
  MongoDB history, Centrifugo realtime, docker-compose, nginx, health probes.
* **Identity**: Telegram Mini App login (init-data HMAC), sessions, account
  settings.
* **Character**: seed-generated LPC appearance, hidden Root/Talent/Potential,
  attributes, levels, XP, class awakening from behaviour at level 10.
* **World**: seed-only worlds, biomes, props, starter plaza, danger gradient,
  chunk streaming, generated species per world.
* **Combat**: server-authoritative melee/ranged/magic, crits, life steal,
  counterattacks, death and respawn, elites and bosses.
* **Items**: procedural equipment (24 bases, 6 rarities, affixes, legendary
  names, mythic traits), item leveling, +N enhancement with an audited
  ledger, salvage, equipment visible on the character.
* **Dungeons**: per-entrance seeded layouts, multi-floor, chests, bosses,
  return-bound exits.
* **History**: chronicle and world firsts.
* **Sprite service**: LPC compositor with palette recolouring, procedural
  creatures/icons/tiles.
* **Hot state in Lua**: GCRA limits, movement token bucket, exactly-once
  kills with threat tables, lazy regen/respawn, sliding-window counters.
* **Weapon effects**: elements with family resistances, glow and particle
  tiers on enhanced/elemental weapons.
* **Economy**: NFTs on OMM Chain (signed blocks, Merkle proofs), player
  market in TON or gold by rarity policy, custodial TON wallet with
  reviewed withdrawals.
* **Admin panel**: realtime Persian web dashboard, moderation, grants,
  withdrawal review, policy, chain explorer, audit log.
* **Client**: Godot web client, full screen in Telegram, responsive
  portrait/landscape, safe areas, touch controls, tap-to-move, customisable
  HUD synced to the account.

## Next

1. **Parties and shared dungeons**: party service; instances owned by a
   party; area channel for the party.
2. **Monster AI tick**: a zone simulation service (aggro, chase, leash,
   ranged projectiles) that publishes monster movement to area channels.
3. **Knowledge / Mastery / Discovery** (`docs/design/KNOWLEDGE_SYSTEM.md`,
   `DISCOVERY_ENGINE.md`): concept graph, experimental crafting through the
   same seeded engine, world-first discoveries.
4. **Creation and crafting** (`CREATION_SYSTEM.md`): recipes, materials from
   biomes, specialists; reuse `items.GenerateBase` for crafted outputs.
5. **Economy**: auctions and contracts on top of the asset service; TON
   Connect for self-custody deposits; Telegram Stars for in-app purchases;
   live testnet run of the TON bridge before mainnet.
6. **NPCs and AI agents** (`AI_AGENTS.md`): seeded NPC residents (the LPC
   `RandomRecipe` already dresses them), teachers, merchants, goals.
7. **Cities and Kingdoms** (`CITY_SYSTEM.md`, `KINGDOM_SYSTEM.md`): player
   founded settlements on the overworld, buildings, treasury, politics, war.
8. **Teleport gates and multiple worlds/realms** (`TELEPORT_GATES.md`,
   `WORLD_SYSTEM.md`): world-service already supports many worlds.
9. **Content pipeline**: hand-made OpenGameArt tilesets and monster sheets
   next to the procedural fallbacks; autotiling transitions between biomes.
10. **Ops**: Kubernetes manifests, OpenTelemetry tracing, Prometheus metrics,
    load tests with `tools/bot -n`.
