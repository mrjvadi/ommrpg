// Package names generates pronounceable fantasy names from a seed. Every
// generated world, species, legendary item and NPC gets its name here so the
// world "names itself" without any stored content.
package names

import (
	"strings"

	"github.com/mrjvadi/ommrpg/backend/pkg/seed"
)

var (
	onsets  = []string{"b", "br", "c", "ch", "d", "dr", "f", "g", "gr", "h", "k", "kr", "l", "m", "n", "p", "r", "s", "sh", "st", "t", "th", "tr", "v", "vr", "z", "zh", "y", "w", "x", "q"}
	vowels  = []string{"a", "e", "i", "o", "u", "ae", "ai", "au", "ei", "ia", "io", "oa", "ou", "y"}
	codas   = []string{"", "", "", "n", "r", "l", "s", "th", "k", "m", "x", "sh", "nd", "rn", "ll", "st"}
	endings = []string{"", "", "ar", "or", "is", "us", "ia", "en", "oth", "ul", "ax", "yr", "eth", "ion", "ra"}
)

// Word builds a single name of the given syllable count.
func Word(r *seed.Rand, syllables int) string {
	var b strings.Builder
	for i := 0; i < syllables; i++ {
		if i > 0 || r.Chance(0.8) {
			b.WriteString(seed.Pick(r, onsets))
		}
		b.WriteString(seed.Pick(r, vowels))
		if i == syllables-1 || r.Chance(0.3) {
			b.WriteString(seed.Pick(r, codas))
		}
	}
	b.WriteString(seed.Pick(r, endings))
	return Capitalize(b.String())
}

// Name returns a 2–3 syllable name derived from s.
func Name(s uint64) string {
	r := seed.New(s)
	return Word(r, r.Range(2, 3))
}

func Capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
