package evalgen_test

import (
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// TestSampleCellsFollowsTheProfile draws 200 of the 1,728 cells at seed 7
// from a profile that saw only five layers and depth two. Uniformly, a third
// of the cells have five layers and a quarter depth two; the profile's
// weights (41/43 and 41/44 of each dimension) must lift both far above that.
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
