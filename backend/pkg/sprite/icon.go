package sprite

import (
	"image"
	"image/color"
	"strings"

	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// Item icon patterns (16x16 logical). Codes: 'M' main material, 'L' main
// highlight, 'H' handle/wood, 'G' gem (rarity colour), 'D' dark detail.
var iconPatterns = map[string][]string{
	"sword": {
		".............LM.", "............LMM.", "...........LMM..", "..........LMM...",
		".........LMM....", "........LMM.....", ".......LMM......", "..G...LMM.......",
		"..GG.LMM........", "...GGMM.........", "....GG..........", "...HGGG.........",
		"..HH..GG........", ".HH.............", "HH..............", "................",
	},
	"axe": {
		"........MMM.....", ".......MMMMM....", "......MMMMMMM...", "......MMLMMMM...",
		".......MLLMM....", "......HHMMM.....", ".....HH..M......", "....HH..........",
		"...HH...........", "..HH............", ".HH.............", "GH..............",
	},
	"mace": {
		"..........M.M...", ".........MMMMM..", "........MMLMMMM.", ".........MLLMM..",
		"........MMMMMM..", ".........MMMM...", "........HH.M....", ".......HH.......",
		"......HH........", ".....HH.........", "....HH..........", "...HH...........",
		"..GH............",
	},
	"spear": {
		"..............LM", ".............LMM", "............LMM.", "...........GMM..",
		"..........HG....", ".........HH.....", "........HH......", ".......HH.......",
		"......HH........", ".....HH.........", "....HH..........", "...HH...........",
		"..HH............", ".HH.............",
	},
	"dagger": {
		"..........LM....", ".........LMM....", "........LMM.....", ".......LMM......",
		"......LMM.......", "....G.MM........", ".....GG.........", "....HGG.........",
		"...HH..G........", "..HH............",
	},
	"bow": {
		"......HH........", ".....HDH........", ".....H..D.......", "....H....D......",
		"....H.....D.....", "...H.......D....", "...H.......G....", "...H.......D....",
		"....H.....D.....", "....H....D......", ".....H..D.......", ".....HDH........",
		"......HH........",
	},
	"crossbow": {
		"....HHHHHHHH....", "...H...MM...H...", "..H....MM....H..", "..D....MM....D..",
		"...DD..MM..DD...", ".....DDGGDD.....", ".......HH.......", ".......HH.......",
		".......HH.......", "......HHHH......",
	},
	"staff": {
		"............GG..", "...........GLGG.", "..........HGGG..", "..........HH....",
		".........HH.....", "........HH......", ".......HH.......", "......HH........",
		".....HH.........", "....HH..........", "...HH...........", "..HH............",
		".HH.............",
	},
	"wand": {
		"...........G....", "..........GLG...", "...........G....", "..........H.....",
		".........H......", "........H.......", ".......H........", "......H.........",
		".....H..........",
	},
	"shield": {
		"..MMMMMMMMMMMM..", ".MLLLLMMMMMMMMM.", ".MLMMMMMMMMMMMM.", ".MLMMMMGGMMMMMM.",
		".MLMMMGGGGMMMMM.", ".MMMMMMGGMMMMMM.", ".MMMMMMMMMMMMMM.", "..MMMMMMMMMMMM..",
		"..MMMMMMMMMMMM..", "...MMMMMMMMMM...", "....MMMMMMMM....", ".....MMMMMM.....",
		"......MMMM......",
	},
	"tome": {
		"...MMMMMMMMMM...", "..MLLLLLLLLLMD..", "..MLMMMMMMMMMD..", "..MLMMMGGMMMMD..",
		"..MLMMGGGGMMMD..", "..MLMMMGGMMMMD..", "..MLMMMMMMMMMD..", "..MLMMMMMMMMMD..",
		"..MMMMMMMMMMMD..", "...DDDDDDDDDD...",
	},
	"helm": {
		".....MMMMMM.....", "....MLLLMMMM....", "...MLMMMMMMMM...", "...MLMMGMMMMM...",
		"...MMMMMMMMMM...", "...MMDDDDDDMM...", "...MMD....DMM...", "...MMD....DMM...",
		"...MMMM..MMMM...", "....MMM..MMM....",
	},
	"hood": {
		".....MMMMMM.....", "....MLLMMMMM....", "...MLMMMMMMMM...", "..MLMMDDDDMMMM..",
		"..MMMD....DMMM..", "..MMD......DMM..", "..MMD......DMM..", "..MMMD....DMMM..",
		"...MMMMMMMMMM...", "....MMMMMMMM....",
	},
	"plate": {
		"...MMM....MMM...", "..MLLMMMMMMMMM..", ".MLLMMMMMMMMMMM.", ".MLMMMMGGMMMMMM.",
		".MMMMMMMMMMMMMM.", "..MLMMMMMMMMMM..", "...MMMMMMMMMM...", "...MLMMDDMMMM...",
		"...MMMMDDMMMM...", "...MMMMMMMMMM...", "....MMMMMMMM....",
	},
	"leather": {
		"...MMM....MMM...", "..MLLMMDDMMMMM..", ".MLLMMMDDMMMMMM.", ".MLMMMMDDMMMMMM.",
		".MMMMMMDDMMMMMM.", "..MLMMMDDMMMMM..", "...MMMMGGMMMM...", "...MLMMDDMMMM...",
		"...MMMMDDMMMM...", "....MMMMMMMM....",
	},
	"robe": {
		"....MMM..MMM....", "...MLLMMMMMMM...", "..MLMMMMGMMMMM..", "..MLMMMMMMMMMM..",
		"...MMMMMMMMMM...", "...MLMMMMMMMM...", "...MLMMMMMMMM...", "..MLMMMMMMMMMM..",
		"..MMMMMMMMMMMM..", ".MMMMMMMMMMMMMM.", ".MMMMMMMMMMMMMM.", ".DDDDDDDDDDDDDD.",
	},
	"greaves": {
		"...MMMMMMMMMM...", "...MLMMMMMMMM...", "...MLMMGGMMMM...", "...MLMM..MMMM...",
		"...MLMM..MMMM...", "...MMMM..MMMM...", "...MLMM..MMMM...", "...MMMM..MMMM...",
		"...MMMM..MMMM...", "..MMMMM..MMMMM..",
	},
	"pants": {
		"...MMMMMMMMMM...", "...MLMMDDMMMM...", "...MLMMMMMMMM...", "...MLMM..MMMM...",
		"...MLMM..MMMM...", "...MMMM..MMMM...", "...MLMM..MMMM...", "...MMMM..MMMM...",
		"...DDDD..DDDD...",
	},
	"boots": {
		"...MMM....MMM...", "...MLM....MLM...", "...MLM....MLM...", "...MMM....MMM...",
		"...MMMM...MMMM..", "..MMMMM..MMMMM..", "..DDDDD..DDDDD..",
	},
	"gloves": {
		".....M.M.M......", ".....M.M.M.M....", "....MLMLMLMM....", "....MLMMMMMM.M..",
		"....MLMMMMMMMM..", "....MMMMMMMMM...", ".....MMMMMMM....", ".....GGGGGGG....",
		".....MMMMMMM....",
	},
	"ring": {
		"......GGG.......", ".....GLGGG......", "......GGG.......", ".....MMMMM......",
		"....ML...MM.....", "...ML.....MM....", "...M.......M....", "...M.......M....",
		"...MM.....MM....", "....MM...MM.....", ".....MMMMM......",
	},
	"amulet": {
		"...M.......M....", "....M.....M.....", ".....M...M......", "......M.M.......",
		".......M........", "......MMM.......", ".....MGGGM......", "....MGLGGGM.....",
		".....MGGGM......", "......MMM.......",
	},
}

var metalShapes = map[string]bool{"sword": true, "axe": true, "mace": true, "spear": true, "dagger": true,
	"shield": true, "helm": true, "plate": true, "greaves": true, "ring": true, "amulet": true}

// RarityColor per rarity index (common..mythic).
var RarityColor = []color.NRGBA{
	{200, 200, 200, 255}, {90, 220, 90, 255}, {70, 140, 255, 255},
	{190, 90, 255, 255}, {255, 160, 40, 255}, {255, 70, 110, 255},
}

// IconShapes lists supported icon shapes.
func IconShapes() []string {
	out := make([]string, 0, len(iconPatterns))
	for k := range iconPatterns {
		out = append(out, k)
	}
	return out
}

// ElementColor is the signature colour of each element (shared with the
// client's weapon effects).
var ElementColor = map[string]color.NRGBA{
	"fire": {255, 120, 30, 255}, "frost": {130, 215, 255, 255}, "lightning": {200, 210, 255, 255},
	"poison": {120, 240, 80, 255}, "holy": {255, 240, 150, 255}, "shadow": {160, 80, 255, 255},
}

// IconFX is Icon with an elemental tint: the gem takes the element colour
// and a few sparkles surround the silhouette.
func IconFX(s uint64, shape string, hue float64, rarity int, element string) *image.NRGBA {
	img := Icon(s, shape, hue, rarity)
	ec, ok := ElementColor[element]
	if !ok {
		return img
	}
	r := seed.New(seed.Derive(s, "icon-fx"))
	gem := RarityColor[min(max(rarity, 0), len(RarityColor)-1)]
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i] == gem.R && img.Pix[i+1] == gem.G && img.Pix[i+2] == gem.B && img.Pix[i+3] == 255 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2] = ec.R, ec.G, ec.B
		}
	}
	for k := 0; k < 6; k++ {
		x, y := r.Intn(30)+1, r.Intn(30)+1
		if img.NRGBAAt(x, y).A == 0 {
			img.SetNRGBA(x, y, ec)
		}
	}
	return img
}

// Icon draws a 32x32 item icon.
func Icon(s uint64, shape string, hue float64, rarity int) *image.NRGBA {
	pat, ok := iconPatterns[shape]
	if !ok {
		pat = iconPatterns["ring"]
	}
	if rarity < 0 || rarity >= len(RarityColor) {
		rarity = 0
	}
	r := seed.New(seed.Derive(s, "icon"))
	var main color.NRGBA
	if metalShapes[shape] {
		main = mix(color.NRGBA{165, 170, 180, 255}, hsv(hue, 0.5, 0.9), 0.15+0.1*float64(rarity))
		if rarity >= 4 {
			main = mix(main, color.NRGBA{235, 190, 70, 255}, 0.55)
		}
	} else {
		main = hsv(hue, r.FRange(0.45, 0.75), r.FRange(0.6, 0.85))
	}
	wood := hsv(0.07+r.FRange(-0.02, 0.02), 0.55, r.FRange(0.45, 0.6))
	gem := RarityColor[rarity]
	logical := newCanvas(16, 16)
	off := (16 - len(pat)) / 2
	for y, row := range pat {
		row = strings.TrimRight(row, " ")
		for x := 0; x < len(row) && x < 16; x++ {
			var col color.NRGBA
			switch row[x] {
			case 'M':
				col = shade(main, 1-float64(y)*0.02)
			case 'L':
				col = shade(main, 1.3)
			case 'H':
				col = wood
			case 'G':
				col = gem
			case 'D':
				col = shade(main, 0.55)
			default:
				continue
			}
			logical.set(x, y+off, col)
		}
	}
	logical.outline(color.NRGBA{25, 20, 30, 255})
	out := newCanvas(32, 32)
	if rarity >= 2 {
		// rarity glow: soft coloured halo around the silhouette
		glow := newCanvas(16, 16)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if logical.get(x, y).A == 0 {
					continue
				}
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					if logical.get(x+d[0], y+d[1]).A == 0 {
						g := gem
						g.A = 110
						glow.set(x+d[0], y+d[1], g)
					}
				}
			}
		}
		out.blit(glow.img, 0, 0, 2)
	}
	out.blit(logical.img, 0, 0, 2)
	return out.img
}
