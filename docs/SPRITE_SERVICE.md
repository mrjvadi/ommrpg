# sprite-service: the game's asset generator

sprite-service plays the role of the
[Universal LPC Spritesheet Character Generator](https://github.com/liberatedpixelcup/Universal-LPC-Spritesheet-Character-Generator)
inside the game. It composes character sheets from layered, palette-recoloured
parts, and it procedurally draws everything that has no hand-made art yet
(monsters, item icons, the tileset). All outputs are deterministic, cached and
served on immutable URLs.

## LPC character sheets

### Catalog

`assets/lpc/catalog.json` is generated from the LPC generator's
`sheet_definitions/` and `palette_definitions/` by:

```
git clone --depth 1 https://github.com/liberatedpixelcup/Universal-LPC-Spritesheet-Character-Generator lpc
cd backend && go run ./tools/lpcimport -repo ../lpc -out ../assets/lpc/catalog.json
```

It holds 646 items (bodies, heads, hair, beards, clothes, armour, helmets,
weapons, shields...), their layers (z-order and per-body-type paths), variants,
recolour channels, the palettes (body, hair, cloth, metal, wood, eye) and the
**credits** of every item. The catalog pins the LPC commit it came from.

### Recipe

```json
{
  "body_type": "female",
  "layers": [
    {"item": "body", "colors": ["amber"]},
    {"item": "heads_human_female", "colors": ["amber", "gray"]},
    {"item": "hair_idol", "colors": ["light_brown"]},
    {"item": "torso_armour_leather", "colors": ["brown"]},
    {"item": "weapon_sword_arming", "variant": "steel"}
  ]
}
```

* `variant` selects a file variant (`<dir>/<anim>/<variant>.png`).
* `colors[i]` recolours channel *i*: the item's base palette (e.g. hair
  "orange", body "light", metal "steel") is mapped index by index to the
  target palette with ±1 tolerance, exactly like the LPC generator. Keys may
  be `name`, `version.name` or `material.version.name`.
* Layers are drawn in z order; body types without a path for an item skip it.
  Unknown items are dropped by `Normalize` (also exposed as `sprite.validate`).

### Output layout

The classic LPC universal sheet, 832x1344, 64x64 frames. Rows within a
4-row animation face up, left, down, right:

| animation | first row | rows | frames |
|---|---|---|---|
| spellcast | 0 | 4 | 7 |
| thrust | 4 | 4 | 8 |
| walk | 8 | 4 | 9 |
| slash | 12 | 4 | 6 |
| shoot | 16 | 4 | 13 |
| hurt | 20 | 1 | 6 |

### Assets on demand

The service does not need the whole LPC art set on disk. With
`SPRITE_REMOTE=default` it lazily downloads the PNGs it needs from the
catalog's pinned commit and caches them under `SPRITE_CACHE_DIR`. Missing
animations are remembered for 24 h. For offline or air-gapped deployments,
set `SPRITE_REMOTE=off` and copy the LPC `spritesheets/` folder to
`$SPRITE_CACHE_DIR/spritesheets`.

## Procedural art

| endpoint | what | parameters |
|---|---|---|
| `creature.png` | 4-frame idle strip, 32x32 frames (48x48 when big) | `seed, family, hue, hue2, big` |
| `icon.png` | 32x32 item icon, rarity glow | `seed, shape, hue, rarity` |
| `tileset.png` | 15 grounds x 4 variants + 17 objects, 32x32 tiles | `seed` |

Creatures use hand-made half-masks per family (mirrored), seeded body pixels,
shading, eyes, outlines and squash/bob frames. Icons use per-shape pixel
patterns tinted by material (metal vs cloth), hue and rarity.

## HTTP API (behind the gateway at `/api/v1/sprites/...`)

| method | path | |
|---|---|---|
| GET | `/v1/sprites/catalog` | items, body types, colour options per channel |
| GET | `/v1/sprites/layout` | sheet layout |
| GET | `/v1/sprites/random?seed=&body=` | seeded random recipe + its URL |
| GET | `/v1/sprites/character.png?r=<base64url(recipe JSON)>` | the sheet |
| POST | `/v1/sprites/character` | normalise a recipe → `{recipe, url, hash, credits}` |
| GET | `/v1/sprites/credits?r=` | attribution for a recipe |
| GET | `/v1/sprites/creature.png`, `/icon.png`, `/tileset.png`, `/tileset.json` | procedural art |

NATS: `sprite.random`, `sprite.validate` (used by character-service).

## Licensing

LPC art is CC-BY-SA / CC-BY / OGA-BY / GPL / CC0 per item. The game must credit
every author of the parts it shows. The catalog stores the credits per item,
and `/credits` returns them for a recipe. See [ASSETS.md](ASSETS.md).
