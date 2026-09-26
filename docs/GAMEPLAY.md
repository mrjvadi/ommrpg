# Gameplay rules (current implementation)

Numbers live in `backend/pkg/game`. This page summarises them.

## Character

* Starts at level 1 with Strength/Agility/Intellect/Vitality 5 each. The
  hidden Root adds bonuses (e.g. Starforged +2 STR +2 VIT).
* **XP to next level** = `60 * level^1.65`. Each level gives 4 free
  attribute points (Hero panel). Max level 100.
* **Derived stats**: HP = 60 + 12·VIT + 8·level + gear; attack uses STR
  (melee), AGI (ranged) or INT (magic) plus weapon damage; crit = 5% +
  0.3%·AGI + gear; attack cooldown shrinks with AGI; move speed 4.5 tiles/s
  (+ gear, capped at +35%).
* **Hidden Talents**: Quick Learner (+15% XP), Fortune Touched (loot luck),
  Blacksmith Precision (+6% enhancement success), Perfect Synthesis,
  Ironhide (defense)...
* **Class awakening at level 10**: never chosen, decided by behaviour. Kill
  counts by weapon kind, upgrades, salvages, deaths and dungeon clears give
  Warrior, Warden, Ranger, Mage, Spellblade or Artisan. Rare Roots with
  matching behaviour unlock hidden paths: Starbreaker, Void Walker,
  Soulbinder, Beastlord.

## Combat

* Server-authoritative: range check against the monster's spawn tile, a
  weapon cooldown, and damage = `attack · U(0.85, 1.15) · (1.75 if crit) ·
  60 / (60 + defense)`.
* Monsters strike back when you are within their reach (their own
  cooldown). Elites have 2.6x HP; bosses 9x.
* HP regenerates 2% per second. Dying sends you to the shrine (or out of the
  dungeon to its entrance). You keep your items.
* Killing monsters far below your level gives much less XP.
* Normal monsters respawn after 45 s, elites after 3 min. Bosses do not
  respawn inside their instance.

## Loot and items

* Normal kills: 30% chance of an item (more with luck), gold, sometimes
  essence. Elites: 1-2 items (at least uncommon). Bosses: 3-5 items (at least
  rare). Chests: 1-3 items.
* Rarity weights: common 60, uncommon 26, rare 10, epic 3.2, legendary 0.7,
  mythic 0.1 (luck raises the higher tiers).
* The bag holds 60 items; drops beyond that are salvaged automatically.

### Items level up

Equipped items gain half of every kill's XP. Each item level gives +3.5% to
its stats, up to a rarity cap (common 10 … mythic 50).

### Enhancement (+N)

At the anvil or from the bag: pay gold and essence to try +1. Success
chance: +1 and +2 always, then 95%, 90%, 80% … down to 4% at +20. Failing at
+7 or higher drops one level; items are never destroyed. The maximum is
+5 (common) to +20 (mythic). Each + adds 7% to the item's stats. Every roll is
logged (`enhance_log`) and reproducible from the server secret.

### Salvage

Destroy an unequipped item for essence and gold (more for rarity, item
level and enhancement).

## World and dungeons

* Danger grows with distance from the starter plaza. Biomes host different
  species.
* Dungeon entrances (dark arches) lead to 1-5 floor dungeons. Walk onto the
  stairs to descend. The last floor has a boss; killing it clears the
  dungeon (a world first the first time anyone clears that entrance).
  Chests open once per run. Leave through the exit next to the start; you
  always come back out at the entrance you used.

## History

The chronicle records notable events, and **world firsts** (first to
level 5/10/15..., first of each class, first epic/legendary/mythic drop of
each base, first +10/+15 enhancement, first clear of each dungeon, first
kill of each elite or boss species) are permanent and announced to everyone.
