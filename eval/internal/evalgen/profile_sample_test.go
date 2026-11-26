package evalgen_test

import (
	"errors"
	"strings"
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

func TestReadProfileRefusesWhatItCannotRead(t *testing.T) {
	raw := profileJSON(t, map[string]int{"3": 1}, map[string]int{}, map[string]int{"function": 1, "method": 0, "http_handler": 0}, nil)
	cases := map[string]struct {
		raw      []byte
		validate func([]byte) error
		reason   string
	}{
		"the validator rejects it": {raw, func([]byte) error { return errors.New("closed schema") }, "closed schema"},
		"another extractor":        {[]byte(strings.Replace(string(raw), `"0.2.0"`, `"0.1.0"`, 1)), acceptAll, `extractor_version "0.1.0"`},
		"another schema version":   {[]byte(strings.Replace(string(raw), `"schema_version":"1"`, `"schema_version":"2"`, 1)), acceptAll, `schema_version "2"`},
	}
	for name, c := range cases {
		if _, err := evalgen.ReadProfile(c.raw, c.validate); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Fatalf("%s: got %v, want an error naming %s", name, err, c.reason)
		}
	}
}

// TestKeyWithoutAProfileIsUnchanged pins the cache key of a fixed cell, so a
// run without a profile reuses every cache entry it had (AC-3.5).
func TestKeyWithoutAProfileIsUnchanged(t *testing.T) {
	cell, _ := evalgen.CellFromIndex(1000)
	const before = "9c743267c7a0c58fbf88575a3e8be6c24ea0eadacf5dc6a766259d64125a6c71"
	if got := evalgen.Key(evalgen.LanguageGo, cell, 7, "tuning", ""); got != before {
		t.Fatalf("the key without a profile is %s, it was %s", got, before)
	}
	if evalgen.Key(evalgen.LanguageGo, cell, 7, "tuning", "abc") == before {
		t.Fatal("a profile run shares the key of a run without one")
	}
}
