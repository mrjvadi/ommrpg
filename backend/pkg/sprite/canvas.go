package sprite

import (
	"image"
	"image/color"
	"math"
)

// hsv converts h,s,v in [0,1] to an opaque colour.
func hsv(h, s, v float64) color.NRGBA {
	h = math.Mod(h, 1)
	if h < 0 {
		h++
	}
	s, v = clamp01(s), clamp01(v)
	i := math.Floor(h * 6)
	f := h*6 - i
	p, q, t := v*(1-s), v*(1-f*s), v*(1-(1-f)*s)
	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return color.NRGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
}

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }

func shade(c color.NRGBA, f float64) color.NRGBA {
	m := func(v uint8) uint8 { return uint8(math.Max(0, math.Min(255, float64(v)*f))) }
	return color.NRGBA{m(c.R), m(c.G), m(c.B), c.A}
}

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

// canvas is a small pixel-art drawing surface.
type canvas struct{ img *image.NRGBA }

func newCanvas(w, h int) *canvas { return &canvas{image.NewNRGBA(image.Rect(0, 0, w, h))} }

func (c *canvas) set(x, y int, col color.NRGBA) {
	if image.Pt(x, y).In(c.img.Rect) {
		c.img.SetNRGBA(x, y, col)
	}
}

func (c *canvas) get(x, y int) color.NRGBA {
	if !image.Pt(x, y).In(c.img.Rect) {
		return color.NRGBA{}
	}
	return c.img.NRGBAAt(x, y)
}

func (c *canvas) rect(x0, y0, x1, y1 int, col color.NRGBA) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c.set(x, y, col)
		}
	}
}

// ellipse fills an ellipse, shading from a light top-left to a dark bottom-right.
func (c *canvas) ellipse(cx, cy, rx, ry float64, col color.NRGBA, shaded bool) {
	for y := int(cy - ry - 1); y <= int(cy+ry+1); y++ {
		for x := int(cx - rx - 1); x <= int(cx+rx+1); x++ {
			dx, dy := (float64(x)+0.5-cx)/rx, (float64(y)+0.5-cy)/ry
			d := dx*dx + dy*dy
			if d > 1 {
				continue
			}
			k := col
			if shaded {
				l := -0.45*dx - 0.55*dy // light from top-left
				f := 1 + l*0.28
				if d > 0.75 {
					f -= 0.12
				}
				k = shade(col, f)
			}
			c.set(x, y, k)
		}
	}
}

func (c *canvas) line(x0, y0, x1, y1 int, col color.NRGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		c.set(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// outline draws a dark 1px border around every opaque region.
func (c *canvas) outline(col color.NRGBA) {
	b := c.img.Rect
	var pts []image.Point
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c.get(x, y).A != 0 {
				continue
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if c.get(x+d[0], y+d[1]).A > 128 {
					pts = append(pts, image.Pt(x, y))
					break
				}
			}
		}
	}
	for _, p := range pts {
		c.set(p.X, p.Y, col)
	}
}

// blit copies src into c at (ox,oy) scaled by an integer factor.
func (c *canvas) blit(src *image.NRGBA, ox, oy, scale int) {
	b := src.Rect
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := src.NRGBAAt(x, y)
			if p.A == 0 {
				continue
			}
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					c.set(ox+(x-b.Min.X)*scale+sx, oy+(y-b.Min.Y)*scale+sy, p)
				}
			}
		}
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
