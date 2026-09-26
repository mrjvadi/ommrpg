package noise

import "testing"

func TestFBMRangeAndDeterminism(t *testing.T) {
	lo, hi := 1.0, 0.0
	for x := -50; x < 50; x++ {
		for y := -50; y < 50; y++ {
			v := FBM(99, float64(x)*0.07, float64(y)*0.07, 5, 2, 0.5)
			if v != FBM(99, float64(x)*0.07, float64(y)*0.07, 5, 2, 0.5) {
				t.Fatal("not deterministic")
			}
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if lo < 0 || hi > 1 || hi-lo < 0.3 {
		t.Fatalf("unexpected spread [%f,%f]", lo, hi)
	}
}
