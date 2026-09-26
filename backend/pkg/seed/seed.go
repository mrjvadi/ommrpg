// Package seed is the single deterministic randomness primitive used by every
// generator in the game (worlds, chunks, monsters, items, dungeons, names,
// sprites). The same (seed, inputs) always produce the same output on every
// service and every platform, so generated content never has to be stored:
// only seeds are persisted and everything else is recomputed on read.
package seed

import (
	"encoding/binary"
	"hash/fnv"
	"math"
)

// splitmix64 finaliser. Good avalanche, tiny and fast.
func mix(z uint64) uint64 {
	z += 0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Derive creates a child seed from a parent seed and a label/path. It is how
// independent streams are carved out of a world seed, e.g.
// Derive(world, "chunk", cx, cy) or Derive(item, "affix", 2).
func Derive(parent uint64, parts ...any) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], parent)
	h.Write(buf[:])
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			h.Write([]byte(v))
		case int:
			binary.LittleEndian.PutUint64(buf[:], uint64(int64(v)))
			h.Write(buf[:])
		case int32:
			binary.LittleEndian.PutUint64(buf[:], uint64(int64(v)))
			h.Write(buf[:])
		case int64:
			binary.LittleEndian.PutUint64(buf[:], uint64(v))
			h.Write(buf[:])
		case uint64:
			binary.LittleEndian.PutUint64(buf[:], v)
			h.Write(buf[:])
		case uint32:
			binary.LittleEndian.PutUint64(buf[:], uint64(v))
			h.Write(buf[:])
		default:
			panic("seed.Derive: unsupported part type")
		}
		h.Write([]byte{0x1f}) // separator so ("ab","c") != ("a","bc")
	}
	return mix(h.Sum64())
}

// FromString hashes an arbitrary string (e.g. a UUID) into a seed.
func FromString(s string) uint64 { return Derive(0, s) }

// Rand is a deterministic splitmix64 stream. The zero value is usable.
type Rand struct{ state uint64 }

func New(s uint64) *Rand { return &Rand{state: s} }

func (r *Rand) Uint64() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Float returns a value in [0,1).
func (r *Rand) Float() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

// Intn returns a value in [0,n). n<=0 returns 0.
func (r *Rand) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.Uint64() % uint64(n))
}

// Range returns an int in [lo,hi] inclusive.
func (r *Rand) Range(lo, hi int) int {
	if hi < lo {
		lo, hi = hi, lo
	}
	return lo + r.Intn(hi-lo+1)
}

// FRange returns a float in [lo,hi).
func (r *Rand) FRange(lo, hi float64) float64 { return lo + r.Float()*(hi-lo) }

// Chance returns true with probability p.
func (r *Rand) Chance(p float64) bool { return r.Float() < p }

// Normal returns a normally distributed value (Box–Muller).
func (r *Rand) Normal(mean, stddev float64) float64 {
	u1 := r.Float()
	if u1 < 1e-12 {
		u1 = 1e-12
	}
	u2 := r.Float()
	return mean + stddev*math.Sqrt(-2*math.Log(u1))*math.Cos(2*math.Pi*u2)
}

// Pick returns a random element of s.
func Pick[T any](r *Rand, s []T) T { return s[r.Intn(len(s))] }

// Weighted picks an index according to non-negative weights.
func (r *Rand) Weighted(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return 0
	}
	x := r.Float() * total
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if x < w {
			return i
		}
		x -= w
	}
	return len(weights) - 1
}

// Shuffle permutes s in place.
func Shuffle[T any](r *Rand, s []T) {
	for i := len(s) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
