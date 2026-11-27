package evalgen_test

import (
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

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
	totals := dimensionTotals(profile)
	for _, group := range [][]string{
		{"packages=2", "packages=3", "packages=4"},
		{"entry=function", "entry=method", "entry=http_handler", "entry=command"},
		{"depth=2", "depth=3", "depth=4", "depth=5"},
		{"field=scalar", "field=collection", "field=nested"},
		{"direction=read", "direction=write"},
		{"trap=layering", "trap=complexity"},
	} {
		expectEqualTotals(t, totals, group)
	}
	if !(totals["layers=3"] > totals["layers=4"]) || !closeTo(totals["layers=4"], totals["layers=5"]) {
		t.Fatalf("layers weigh %v, %v, %v; 3 was observed and 9 is out of range", totals["layers=3"], totals["layers=4"], totals["layers=5"])
	}
}

// dimensionTotals is the summed weight of every cell holding each value of
// each dimension, keyed "dimension=value".
func dimensionTotals(profile evalgen.Profile) map[string]float64 {
	totals := map[string]float64{}
	for index := 0; index < evalgen.CellCount; index++ {
		cell, _ := evalgen.CellFromIndex(index)
		weight := profile.Weight(cell)
		for _, key := range []string{"layers=" + itoa(cell.Layers), "packages=" + itoa(cell.Packages), "entry=" + cell.Entry,
			"depth=" + itoa(cell.Depth), "field=" + cell.Field, "direction=" + cell.Direction, "trap=" + cell.Trap} {
			totals[key] += weight
		}
	}
	return totals
}

func expectEqualTotals(t *testing.T, totals map[string]float64, group []string) {
	t.Helper()
	for _, key := range group[1:] {
		if !closeTo(totals[key], totals[group[0]]) {
			t.Fatalf("%s weighs %v but %s weighs %v; an unset dimension is uniform", key, totals[key], group[0], totals[group[0]])
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
