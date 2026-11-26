package evalgen_test

import (
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// outOfRangeProfile is the skewed profile plus n observations of each
// dimension at values no cell holds (9 layers, 7 packages, depth 9), as a
// real repository's profile carries.
func outOfRangeProfile(t *testing.T, n int) evalgen.Profile {
	return readProfile(t, profileJSON(t, map[string]int{"5": 40, "9": n}, map[string]int{"2": 40, "9": n},
		map[string]int{"function": 0, "method": 0, "http_handler": 0},
		map[string]map[string]int{"core": {"7": n}}))
}

// TestSampleCellsKeepsTheSeedAndTheSkewAtAnyProfileScale guards the draw
// against underflow: observations outside the generator's range make every
// weight small together, and the draw must still follow the seed and the
// in-range skew rather than collapse to the first cells by index. Scaling
// every weight alike leaves the order of the keys as it was, so the draw at
// seed 7 is the skewed profile's.
func TestSampleCellsKeepsTheSeedAndTheSkewAtAnyProfileScale(t *testing.T) {
	skewed, _ := evalgen.SampleCells(skewedProfile(t), 200, 7)
	for _, n := range []int{10, 100000} {
		profile := outOfRangeProfile(t, n)
		first, _ := evalgen.SampleCells(profile, 100, 1)
		second, _ := evalgen.SampleCells(profile, 100, 2)
		if equalCells(first, second) {
			t.Fatalf("at %d out-of-range observations, seeds 1 and 2 drew the same cells", n)
		}
		drawn, _ := evalgen.SampleCells(profile, 200, 7)
		fiveLayers := countWhere(drawn, func(c evalgen.Cell) bool { return c.Layers == 5 })
		depthTwo := countWhere(drawn, func(c evalgen.Cell) bool { return c.Depth == 2 })
		if fiveLayers < 160 || depthTwo < 150 {
			t.Fatalf("at %d out-of-range observations, five layers in %d and depth two in %d of 200; the in-range skew is lost", n, fiveLayers, depthTwo)
		}
		if !equalCells(drawn, skewed) {
			t.Fatalf("at %d out-of-range observations, seed 7 drew other cells than the skewed profile alone", n)
		}
	}
}
