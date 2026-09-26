// Command spriterender renders a random (seeded) LPC character sheet or a
// procedural creature to a PNG file. Handy for previewing seeds.
//
//	go run ./tools/spriterender -seed 7 -out char.png
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/mrjvadi/ommrpg/backend/pkg/sprite"
)

func main() {
	s := flag.Uint64("seed", 1, "appearance seed")
	body := flag.String("body", "", "body type (male, female, ...)")
	catalog := flag.String("catalog", "../assets/lpc/catalog.json", "catalog path")
	dir := flag.String("assets", "../assets/lpc/spritesheets", "local spritesheets dir (cache)")
	remote := flag.String("remote", "", "remote base url (defaults to the catalog's pinned LPC commit)")
	out := flag.String("out", "character.png", "output")
	flag.Parse()
	cat, err := sprite.LoadCatalog(*catalog)
	if err != nil {
		log.Fatal(err)
	}
	if *remote == "" {
		*remote = sprite.DefaultRemote(cat)
	}
	comp := &sprite.Composer{Cat: cat, Assets: sprite.NewAssets(*dir, *remote)}
	rec := cat.RandomRecipe(*s, *body)
	log.Printf("recipe: %+v", rec)
	png, err := comp.RenderPNG(context.Background(), rec)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, png, 0o644); err != nil {
		log.Fatal(err)
	}
}
