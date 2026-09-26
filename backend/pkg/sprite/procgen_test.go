package sprite

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func opaque(img *image.NRGBA) int {
	n := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 0 {
			n++
		}
	}
	return n
}

func TestProcgenOutputs(t *testing.T) {
	for _, fam := range CreatureFamilies() {
		img := Creature(42, fam, 0.3, 0.7, false)
		if img.Rect.Dx() != 32*CreatureFrames || img.Rect.Dy() != 32 {
			t.Fatalf("%s: bad size %v", fam, img.Rect)
		}
		if opaque(img) < 200 {
			t.Fatalf("%s: creature nearly empty", fam)
		}
	}
	for _, shape := range IconShapes() {
		if img := Icon(1, shape, 0.5, 3); opaque(img) < 100 {
			t.Fatalf("%s icon nearly empty", shape)
		}
	}
	atlas, meta := Tileset(7)
	if atlas.Rect.Dx() != meta.Columns*TileSize || atlas.Rect.Dy() != (meta.ObjectRow+1)*TileSize {
		t.Fatal("atlas size mismatch")
	}
	// Set SPRITE_PREVIEW_DIR to dump preview sheets for eyeballing.
	dir := os.Getenv("SPRITE_PREVIEW_DIR")
	if dir == "" {
		return
	}
	write := func(name string, img *image.NRGBA) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, img)
	}
	write("tileset.png", atlas)
	fams := []string{"slime", "beast", "insect", "bird", "undead", "elemental", "plant", "golem", "serpent", "imp", "spirit"}
	sheet := newCanvas(32*CreatureFrames*2, 32*len(fams))
	for i, fam := range fams {
		sheet.blit(Creature(uint64(i+3), fam, float64(i)/11, float64(i)/7, false), 0, i*32, 1)
		sheet.blit(Creature(uint64(i+99), fam, float64(i)/5, float64(i)/3, false), 32*CreatureFrames, i*32, 1)
	}
	write("creatures.png", sheet.img)
	shapes := []string{"sword", "axe", "mace", "spear", "dagger", "bow", "crossbow", "staff", "wand", "shield", "tome", "helm", "hood", "plate", "leather", "robe", "greaves", "pants", "boots", "gloves", "ring", "amulet"}
	icons := newCanvas(32*len(shapes), 32*6)
	for i, s := range shapes {
		for rr := 0; rr < 6; rr++ {
			icons.blit(Icon(uint64(i), s, float64(i)/22, rr), i*32, rr*32, 1)
		}
	}
	write("icons.png", icons.img)
}
