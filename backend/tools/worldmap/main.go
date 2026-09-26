// Command worldmap renders a whole generated world to a PNG. Useful for tuning
// the generator and for previewing a seed before creating a world with it.
//
//	go run ./tools/worldmap -seed 42 -out world.png
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"

	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
)

var groundColors = map[world.Ground]color.RGBA{
	world.GDeepWater: {20, 50, 110, 255}, world.GWater: {40, 90, 170, 255},
	world.GSand: {220, 200, 140, 255}, world.GGrass: {90, 160, 70, 255},
	world.GForest: {50, 110, 50, 255}, world.GDirt: {140, 120, 80, 255},
	world.GSnow: {235, 240, 245, 255}, world.GSwamp: {80, 100, 70, 255},
	world.GRock: {120, 115, 110, 255}, world.GAsh: {70, 60, 60, 255},
	world.GPath: {190, 170, 120, 255}, world.GIce: {180, 220, 240, 255},
}

func main() {
	s := flag.Uint64("seed", 123456789, "world seed")
	size := flag.Int("chunks", 64, "world size in chunks")
	out := flag.String("out", "world.png", "output file")
	flag.Parse()
	p := world.Params{ID: 1, Seed: *s, SizeChunks: *size}
	n := p.SizeTiles()
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for cy := 0; cy < *size; cy++ {
		for cx := 0; cx < *size; cx++ {
			t := p.ChunkTerrain(cx, cy)
			for i := range t.Ground {
				c := groundColors[world.Ground(t.Ground[i])]
				switch world.Object(t.Objects[i]) {
				case world.OTree, world.OPine:
					c = color.RGBA{c.R / 2, c.G / 2, c.B / 2, 255}
				case world.ODungeonEntrance:
					c = color.RGBA{255, 0, 255, 255}
				case world.OShrine:
					c = color.RGBA{255, 255, 0, 255}
				}
				img.Set(cx*world.ChunkSize+i%world.ChunkSize, cy*world.ChunkSize+i/world.ChunkSize, c)
			}
		}
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}
