package evalgen_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func acceptAll([]byte) error { return nil }

// profileJSON is a profile in the committed schema's shape with the given
// dimensions; roles carry packages per layer.
func profileJSON(t *testing.T, layers, depth map[string]int, entry map[string]int, roles map[string]map[string]int) []byte {
	t.Helper()
	roleObjects := map[string]any{}
	for role, packages := range roles {
		roleObjects[role] = map[string]any{"packages": packages}
	}
	raw, err := json.Marshal(map[string]any{
		"schema_version": "1", "identity": strings.Repeat("ab", 32), "label": "stand-in",
		"sha": strings.Repeat("0", 40), "licence": "MIT", "extractor_version": "0.2.0",
		"roles": roleObjects,
		"dimensions": map[string]any{
			"layers": layers, "entry": entry, "depth": depth, "trap": nil, "field": nil, "direction": nil,
		},
		"shapes": map[string]any{
			"nesting": map[string]int{}, "cognitive": map[string]int{}, "branch_density": map[string]int{},
			"constructor": map[string]int{"assigns_only": 0, "does_work": 0},
		},
		"not_covered": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func readProfile(t *testing.T, raw []byte) evalgen.Profile {
	t.Helper()
	profile, err := evalgen.ReadProfile(raw, acceptAll)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func skewedProfile(t *testing.T) evalgen.Profile {
	return readProfile(t, profileJSON(t, map[string]int{"5": 40}, map[string]int{"2": 40},
		map[string]int{"function": 0, "method": 0, "http_handler": 0}, nil))
}

func TestEveryCellKeepsAPositiveWeight(t *testing.T) {
	profile := skewedProfile(t)
	for index := 0; index < evalgen.CellCount; index++ {
		cell, _ := evalgen.CellFromIndex(index)
		if weight := profile.Weight(cell); weight <= 0 {
			t.Fatalf("cell %d %+v has weight %v; every cell stays possible (AC-3.1)", index, cell, weight)
		}
	}
}

// TestUnsetDimensionsAreUniform holds layers fixed and checks that, with only
// layers set, each value of every other dimension carries the same total
// weight (AC-3.2), and that an out-of-range layer value counts for nothing.
func TestUnsetDimensionsAreUniform(t *testing.T) {
	profile := readProfile(t, profileJSON(t, map[string]int{"3": 9, "9": 50}, map[string]int{},
		map[string]int{"function": 0, "method": 0, "http_handler": 0}, nil))
	totals := map[string]float64{}
	for index := 0; index < evalgen.CellCount; index++ {
		cell, _ := evalgen.CellFromIndex(index)
		weight := profile.Weight(cell)
		for _, key := range []string{"packages=" + itoa(cell.Packages), "entry=" + cell.Entry, "depth=" + itoa(cell.Depth),
			"field=" + cell.Field, "direction=" + cell.Direction, "trap=" + cell.Trap} {
			totals[key] += weight
		}
		totals["layers="+itoa(cell.Layers)] += weight
	}
	for _, group := range [][]string{
		{"packages=2", "packages=3", "packages=4"},
		{"entry=function", "entry=method", "entry=http_handler", "entry=command"},
		{"depth=2", "depth=3", "depth=4", "depth=5"},
		{"field=scalar", "field=collection", "field=nested"},
		{"direction=read", "direction=write"},
		{"trap=layering", "trap=complexity"},
	} {
		for _, key := range group[1:] {
			if !closeTo(totals[key], totals[group[0]]) {
				t.Fatalf("%s weighs %v but %s weighs %v; an unset dimension is uniform", key, totals[key], group[0], totals[group[0]])
			}
		}
	}
	if !(totals["layers=3"] > totals["layers=4"]) || !closeTo(totals["layers=4"], totals["layers=5"]) {
		t.Fatalf("layers weigh %v, %v, %v; 3 was observed and 9 is out of range", totals["layers=3"], totals["layers=4"], totals["layers=5"])
	}
}

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
	seen, fiveLayers, depthTwo := map[int]bool{}, 0, 0
	for _, cell := range cells {
		if seen[cell.Index()] {
			t.Fatalf("cell %+v was drawn twice; sampling is without replacement", cell)
		}
		seen[cell.Index()] = true
		if cell.Layers == 5 {
			fiveLayers++
		}
		if cell.Depth == 2 {
			depthTwo++
		}
	}
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

func TestPackagesComeFromEveryRole(t *testing.T) {
	profile := readProfile(t, profileJSON(t, map[string]int{}, map[string]int{},
		map[string]int{"function": 0, "method": 0, "http_handler": 0},
		map[string]map[string]int{"api": {"2": 1}, "store": {"2": 1}, "service": {"4": 1}}))
	two, _ := evalgen.CellFromIndex(0)
	four := two
	two.Packages, four.Packages = 2, 4
	if !(profile.Weight(two) > profile.Weight(four)) {
		t.Fatalf("packages 2 (two roles) weighs %v, packages 4 (one role) %v", profile.Weight(two), profile.Weight(four))
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

func equalCells(a, b []evalgen.Cell) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func closeTo(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1e-9*(a+b)
}

func itoa(n int) string { return strings.TrimSpace(string(rune('0' + n))) }
