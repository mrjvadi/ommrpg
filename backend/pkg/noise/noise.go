// Package noise provides seeded, allocation-free 2D gradient noise and fractal
// Brownian motion used by terrain generation. Pure functions of (seed, x, y).
package noise

import "math"

func hash2(seed uint64, x, y int64) uint64 {
	h := seed ^ uint64(x)*0x9e3779b97f4a7c15 ^ uint64(y)*0xc2b2ae3d27d4eb4f
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

func grad(seed uint64, ix, iy int64, dx, dy float64) float64 {
	a := float64(hash2(seed, ix, iy)&0xffff) / 65536.0 * 2 * math.Pi
	return math.Cos(a)*dx + math.Sin(a)*dy
}

func fade(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }

// Perlin returns gradient noise in roughly [-1,1].
func Perlin(seed uint64, x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	ix, iy := int64(x0), int64(y0)
	fx, fy := x-x0, y-y0
	n00 := grad(seed, ix, iy, fx, fy)
	n10 := grad(seed, ix+1, iy, fx-1, fy)
	n01 := grad(seed, ix, iy+1, fx, fy-1)
	n11 := grad(seed, ix+1, iy+1, fx-1, fy-1)
	u, v := fade(fx), fade(fy)
	nx0 := n00 + u*(n10-n00)
	nx1 := n01 + u*(n11-n01)
	return (nx0 + v*(nx1-nx0)) * 1.41421356
}

// FBM sums octaves of Perlin noise and normalises the result into [0,1].
func FBM(seed uint64, x, y float64, octaves int, lacunarity, gain float64) float64 {
	amp, freq, sum, norm := 1.0, 1.0, 0.0, 0.0
	for i := 0; i < octaves; i++ {
		sum += amp * Perlin(seed+uint64(i)*0x632be59bd9b4e019, x*freq, y*freq)
		norm += amp
		amp *= gain
		freq *= lacunarity
	}
	v := (sum/norm)*0.5 + 0.5
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
