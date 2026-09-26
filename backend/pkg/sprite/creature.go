package sprite

import (
	"image"
	"image/color"

	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

// Creature templates: the left half (8 columns) of a 16x16 logical sprite,
// mirrored to the right. Codes: '.' empty, 'o' maybe body, 'X' body,
// 'a' accent colour, 'd' dark part, 'e' eye.
var creatureTemplates = map[string][]string{
	"slime": {
		"........", "........", "........", "........", "........", "......oo",
		"....oXXX", "...oXXXX", "..oXXeXX", "..oXXXXX", ".oXXXXXX", ".oXaXXXX",
		".oXXXXXX", "oXXXXXXX", ".XXXXXXX", "........",
	},
	"beast": {
		"........", "..o.....", "..Xo....", "..XXo.oo", "...XXXXX", "...XXeXX",
		"...XXXXX", "....XXaX", "..oXXXXX", ".oXXXXXX", ".XXXXXXX", ".XXXXXXX",
		".XdXXXXX", ".Xd..XXX", ".dd..dd.", "........",
	},
	"insect": {
		"........", "...o....", "....o...", "....oXXX", "....XeXX", ".....XXX",
		"..o.XXaX", ".oXoXXXX", "o.XXXXXX", "..oXXXXX", ".o.XXaXX", "o..XXXXX",
		"..oXXXXX", ".o..XXXX", "....d..d", "........",
	},
	"bird": {
		"........", "........", "....oXXX", "....XeXX", ".....Xaa", "o...XXXX",
		"Xo..XXXX", "XXo.XXXX", ".XXXXXXX", "..XXXXXX", "...XXXXX", "....XXXX",
		".....d.d", ".....d.d", "........", "........",
	},
	"undead": {
		"........", ".....XXX", "....XXXX", "....XeXX", "....XXXX", ".....X.X",
		"...oXXXX", "..o.XaXX", "..X.XXXX", "..X.XaXX", "....XXXX", "....X..X",
		"....d..d", "....d..d", "...dd..d", "........",
	},
	"elemental": {
		"......o.", ".....oo.", "....oXo.", "...oXXXo", "...XXXXX", "..oXXXXX",
		"..XXeXXX", "..XXXXaX", ".oXXXXXX", ".XXXaXXX", ".XXXXXXX", "..XXXXXX",
		"..oXXXXX", "...oXXXX", ".....oXX", "........",
	},
	"plant": {
		"........", "..oo..oo", ".oXXo.XX", "..oXXoXX", "....XXXX", "...XaaXX",
		"..XXXeXX", "..XaXXXX", "...XXXXX", "....oXXX", ".....XdX", ".....dXd",
		"....dd.d", "...d.d..", "..d..d..", "........",
	},
	"golem": {
		"........", "....XXXX", "....XXXX", "....XeXX", "....XXXX", ".XX.aXXX",
		"XXXXXXXX", "XXX.XXXX", "XX..XaXX", "XX..XXXX", "X...XXXX", "....XX.X",
		"....XX.X", "...XXX.X", "...XXX.X", "........",
	},
	"serpent": {
		"........", "....oXXX", "....XeXX", "....XXXX", ".....aX.", ".....XX.",
		"...oXXX.", "..XXXXXX", ".XXXXaXX", ".XXXXXXX", "..XXXXXX", ".oXXXXXX",
		"XXXXXXXX", ".XXXXXXX", "..oooooo", "........",
	},
	"imp": {
		"........", "..o.....", "..Xo....", "...XoXXX", "....XXXX", "....XeXX",
		".....XXX", "o..XXaXX", "Xo.XXXXX", "XXoXXXXX", "XXXXXXXX", ".X..XXXX",
		"....XX.X", "....X..X", "....d..d", "........",
	},
	"spirit": {
		"........", "........", ".....XXX", "....XXXX", "...XXXXX", "...XeXXX",
		"...XXXXX", "...XXaXX", "..oXXXXX", "..XXXXXX", "..XXXXXX", "..XXXXXX",
		"..XXXXXX", "..X.XX.X", "..o..o..", "........",
	},
}

var flying = map[string]bool{"bird": true, "elemental": true, "spirit": true, "imp": true}

var eyeColor = map[string]color.NRGBA{
	"undead": {255, 60, 40, 255}, "imp": {255, 220, 60, 255}, "spirit": {230, 250, 255, 255},
	"elemental": {255, 255, 210, 255}, "golem": {120, 230, 255, 255},
}

// CreatureFamilies lists supported families.
func CreatureFamilies() []string {
	out := make([]string, 0, len(creatureTemplates))
	for k := range creatureTemplates {
		out = append(out, k)
	}
	return out
}

// CreatureFrames is the number of idle animation frames per creature sheet.
const CreatureFrames = 4

// Creature draws a 4-frame idle strip for a monster species. Each frame is
// 32x32 (or 48x48 when big). Unknown families fall back to "slime".
func Creature(s uint64, family string, hue, hue2 float64, big bool) *image.NRGBA {
	tpl, ok := creatureTemplates[family]
	if !ok {
		tpl = creatureTemplates["slime"]
	}
	r := seed.New(seed.Derive(s, "creature"))
	sat := r.FRange(0.45, 0.8)
	base := hsv(hue, sat, 0.85)
	accent := hsv(hue2, r.FRange(0.5, 0.9), 0.95)
	eye, ok := eyeColor[family]
	if !ok {
		eye = color.NRGBA{250, 250, 250, 255}
	}
	// logical 16x16 sprite
	logical := newCanvas(16, 16)
	for y, row := range tpl {
		for x := 0; x < 8; x++ {
			ch := row[x]
			if ch == 'o' && r.Chance(0.5) {
				ch = 'X'
			}
			var col color.NRGBA
			v := 1.05 - float64(y)*0.03 + r.FRange(-0.05, 0.05)
			switch ch {
			case 'X':
				col = shade(base, v)
			case 'a':
				col = shade(accent, v)
			case 'd':
				col = shade(base, 0.55)
			case 'e':
				col = eye
			default:
				continue
			}
			logical.set(x, y, col)
			logical.set(15-x, y, col)
		}
	}
	// pupils: a dark pixel below/inside each eye for character
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if logical.get(x, y) == eye && family != "spirit" {
				if x < 8 {
					logical.set(x+1, y, shade(base, 0.25))
				} else {
					logical.set(x-1, y, shade(base, 0.25))
				}
			}
		}
	}
	logical.outline(shade(base, 0.22))
	scale := 2
	if big {
		scale = 3
	}
	fs := 16 * scale
	out := newCanvas(fs*CreatureFrames, fs)
	for f := 0; f < CreatureFrames; f++ {
		dy := []int{0, 1, 0, -1}[f]
		fr := newCanvas(16, 16)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				p := logical.get(x, y)
				if p.A == 0 {
					continue
				}
				ty := y
				if flying[family] {
					ty = y + dy // whole-body bob
				} else if y < 9 {
					ty = y + dy // squash/stretch the upper body
				}
				fr.set(x, ty, p)
				if !flying[family] && y < 9 && dy < 0 && y == 8 {
					fr.set(x, 8, p) // fill the gap when stretching
				}
			}
		}
		out.blit(fr.img, f*fs, 0, scale)
	}
	return out.img
}
