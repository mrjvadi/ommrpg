# Art assets and licensing

## Sources

* **Characters and equipment**: the Liberated Pixel Cup (LPC) collection from
  [OpenGameArt.org](https://opengameart.org/content/lpc-collection), consumed
  through the
  [Universal LPC Spritesheet Character Generator](https://github.com/liberatedpixelcup/Universal-LPC-Spritesheet-Character-Generator)
  repository (its `sheet_definitions`, `palette_definitions` and
  `spritesheets`). See [SPRITE_SERVICE.md](SPRITE_SERVICE.md).
* **Monsters, item icons, tiles**: generated procedurally by sprite-service
  (original work of this project), so the game is fully playable without any
  extra downloads.
* **Font**: [Vazirmatn](https://github.com/rastikerdar/vazirmatn) (SIL Open
  Font License 1.1, `client/assets/fonts/OFL.txt`), chosen because it covers
  both Persian and Latin script.

## Attribution duties

LPC parts are licensed per item (CC-BY-SA 3.0/4.0, CC-BY, OGA-BY, GPL 2.0/3.0,
CC0). You must credit every author of every part shown in the game:

* The catalog keeps the `credits` (authors, licenses, source URLs) of each
  item.
* `GET /api/v1/sprites/credits?r=<recipe>` returns the credits of a character.
* Ship a credits screen that links to the LPC generator's
  [CREDITS.csv](https://github.com/liberatedpixelcup/Universal-LPC-Spritesheet-Character-Generator/blob/master/CREDITS.csv)
  or to a filtered list of the parts you use (see the LPC README's
  "Licensing and Attribution" section).
* CC-BY-SA requires derivative art (for example recoloured sheets) to stay
  CC-BY-SA; do not DRM-protect it.

## Adding more OpenGameArt content

The pipeline is data-driven, so adding art rarely needs code:

1. **More LPC wearables**: add them to the LPC generator format (or pull a
   newer LPC commit) and re-run `tools/lpcimport`. New items are immediately
   usable in recipes; map them to item bases in `pkg/game/items` (`Looks`)
   to make them drop as equipment.
2. **Hand-made tiles** (e.g. LPC terrain/"Atlas" tilesets): the client indexes
   the atlas as `(variant, ground_id)` for ground and `(object_id, 15)` for
   objects, 32x32 each (`/api/v1/sprites/tileset.json`). Produce an atlas
   with that layout and serve it instead of the procedural one.
3. **Hand-made monsters**: sprite-service can serve any 4-frame strip in
   place of `creature.png`. Keep the per-species `sprite_seed`/`family`
   keys as the lookup, and keep an attribution list for everything you add.
