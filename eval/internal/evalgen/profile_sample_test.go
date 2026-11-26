package evalgen_test

import (
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// TestSampleCellsFollowsTheProfile draws 200 of the 1,728 cells at seed 7
// from a profile that saw only five layers and depth two. Uniformly, a third
// of the cells have five layers and a quarter depth two (about 67 and 50 of
// 200); the profile's weights (41/43 and 41/44 of each dimension) must lift
// both to at least 80% and 75%. The draw is deterministic at a fixed seed,
// so the exact counts are pinned too.
func TestSampleCellsFollowsTheProfile(t *testing.T) {
	cells, err := evalgen.SampleCells(skewedProfile(t), 200, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 200 {
		t.Fatalf("sampled %d cells, want 200", len(cells))
	}
	if !distinct(cells) {
		t.Fatal("a cell was drawn twice; sampling is without replacement")
	}
	fiveLayers := countWhere(cells, func(c evalgen.Cell) bool { return c.Layers == 5 })
	depthTwo := countWhere(cells, func(c evalgen.Cell) bool { return c.Depth == 2 })
	if fiveLayers < 160 || depthTwo < 150 {
		t.Fatalf("five layers in %d and depth two in %d of 200; the profile weights them over uniform's ~67 and ~50", fiveLayers, depthTwo)
	}
	if fiveLayers != 180 || depthTwo != 156 {
		t.Fatalf("five layers in %d and depth two in %d of 200 at seed 7; the draw is deterministic and was 180 and 156", fiveLayers, depthTwo)
	}
}

func TestSampleCellsIsReproducibleBySeed(t *testing.T) {
	profile := skewedProfile(t)
	first, _ := evalgen.SampleCells(profile, 20, 7)
	again, _ := evalgen.SampleCells(profile, 20, 7)
	other, _ := evalgen.SampleCells(profile, 20, 8)
	if !equalCells(first, again) {
		t.Fatal("the same profile, count and seed drew different cells")
	}
	if equalCells(first, other) {
		t.Fatal("seeds 7 and 8 drew the same cells")
	}
}

func TestSampleCellsRefusesACountOutsideTheCellSpace(t *testing.T) {
	for _, count := range []int{0, -1, evalgen.CellCount + 1} {
		if _, err := evalgen.SampleCells(skewedProfile(t), count, 7); err == nil {
			t.Fatalf("count %d was accepted", count)
		}
	}
	if _, err := evalgen.SampleCells(skewedProfile(t), evalgen.CellCount, 7); err != nil {
		t.Fatalf("every cell could not be drawn: %v", err)
	}
}

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
