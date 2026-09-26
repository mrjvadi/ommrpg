package seed

import "testing"

func TestDeterministic(t *testing.T) {
	a, b := New(Derive(42, "chunk", 3, -7)), New(Derive(42, "chunk", 3, -7))
	for i := 0; i < 1000; i++ {
		if a.Uint64() != b.Uint64() {
			t.Fatal("streams diverged")
		}
	}
}

func TestDeriveSeparatesParts(t *testing.T) {
	if Derive(1, "ab", "c") == Derive(1, "a", "bc") {
		t.Fatal("part boundaries must matter")
	}
	if Derive(1, "x") == Derive(2, "x") {
		t.Fatal("parent must matter")
	}
}

func TestRangesAndWeighted(t *testing.T) {
	r := New(7)
	counts := make([]int, 3)
	for i := 0; i < 30000; i++ {
		v := r.Range(-2, 2)
		if v < -2 || v > 2 {
			t.Fatalf("range out of bounds: %d", v)
		}
		f := r.Float()
		if f < 0 || f >= 1 {
			t.Fatalf("float out of bounds: %f", f)
		}
		counts[r.Weighted([]float64{1, 0, 3})]++
	}
	if counts[1] != 0 {
		t.Fatal("zero weight picked")
	}
	if counts[2] < counts[0]*2 {
		t.Fatalf("weights not respected: %v", counts)
	}
}
