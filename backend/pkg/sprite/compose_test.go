package sprite

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrjvadi/ommrpg/backend/pkg/contracts"
)

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := LoadCatalog("../../../assets/lpc/catalog.json")
	if err != nil {
		t.Skipf("catalog not available: %v", err)
	}
	return c
}

func TestRandomRecipeDeterministicAndValid(t *testing.T) {
	cat := testCatalog(t)
	comp := &Composer{Cat: cat}
	for s := uint64(0); s < 50; s++ {
		a, b := cat.RandomRecipe(s, ""), cat.RandomRecipe(s, "")
		if comp.Hash(a) != comp.Hash(b) {
			t.Fatal("random recipe not deterministic")
		}
		n, dropped := comp.Normalize(a)
		if len(dropped) > 0 {
			t.Fatalf("random recipe produced invalid parts %v", dropped)
		}
		if len(n.Layers) < 4 {
			t.Fatalf("recipe too thin: %+v", n)
		}
	}
}

func TestNormalizeDropsUnknown(t *testing.T) {
	comp := &Composer{Cat: testCatalog(t)}
	r, dropped := comp.Normalize(contracts.Recipe{BodyType: "alien", Layers: []contracts.Layer{{Item: "nope"}, {Item: "weapon_blunt_mace", Variant: "steel"}}})
	if r.BodyType != "male" || len(dropped) != 1 || r.Layers[0].Item != "body" {
		t.Fatalf("bad normalize %+v %v", r, dropped)
	}
	if r.Layers[1].Variant != "mace" {
		t.Fatalf("invalid variant should fall back, got %q", r.Layers[1].Variant)
	}
}

func TestRecolorAndRenderFromLocalAssets(t *testing.T) {
	cat := testCatalog(t)
	dir := t.TempDir()
	// fake a 2-color body walk sheet using the body palette's source colours
	src, _ := cat.sourcePalette(cat.Items["body"].Recolors[0])
	c0, _ := parseHex(src[3])
	img := image.NewNRGBA(image.Rect(0, 0, 9*Frame, 4*Frame))
	img.Set(10, 10, color.NRGBA{c0.r, c0.g, c0.b, 255})
	p := filepath.Join(dir, "body/bodies/male/walk.png")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, _ := os.Create(p)
	_ = png.Encode(f, img)
	f.Close()
	comp := &Composer{Cat: cat, Assets: NewAssets(dir, "")}
	sheet, err := comp.Render(context.Background(), contracts.Recipe{BodyType: "male", Layers: []contracts.Layer{{Item: "body", Colors: []string{"black"}}}})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := cat.resolvePalette("body", "black")
	w, _ := parseHex(want[3])
	got := sheet.NRGBAAt(10, 8*Frame+10)
	if got.R != w.r || got.G != w.g || got.B != w.b {
		t.Fatalf("recolor failed: got %v want %v", got, w)
	}
}

func TestWeaponMask(t *testing.T) {
	cat := testCatalog(t)
	dir := t.TempDir()
	put := func(rel string, px int) {
		img := image.NewNRGBA(image.Rect(0, 0, 9*Frame, 4*Frame))
		img.Set(px, 10, color.NRGBA{200, 200, 200, 255})
		img.Set(40, 40, color.NRGBA{200, 200, 200, 255})
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		f, _ := os.Create(p)
		_ = png.Encode(f, img)
		f.Close()
	}
	put("body/bodies/male/walk.png", 10)
	w := cat.Items["weapon_blunt_mace"]
	put(w.Layers[0].Paths["male"]+"walk/mace.png", 20)
	comp := &Composer{Cat: cat, Assets: NewAssets(dir, "")}
	rec := contracts.Recipe{BodyType: "male", Layers: []contracts.Layer{{Item: "body"}, {Item: "weapon_blunt_mace"}}}
	mask, err := comp.RenderMask(context.Background(), rec, "weapon")
	if err != nil {
		t.Fatal(err)
	}
	row := 8 * Frame
	if mask.NRGBAAt(20, row+10).A != 255 || mask.NRGBAAt(10, row+10).A != 0 {
		t.Fatal("mask should cover only the weapon pixels")
	}
	// (40,40) is drawn by both; the top layer decides the owner
	top := w.Layers[0].Z > 10
	if (mask.NRGBAAt(40, row+40).A == 255) != top {
		t.Fatal("overlap ownership wrong")
	}
}
