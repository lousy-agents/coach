package evalgen_test

import (
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// TestSampleCellsFollowsTheProfilesEntryKinds draws 200 cells at seed 7 from
// a profile that saw only HTTP handlers. Uniformly a quarter of the cells
// (about 50) are handlers; the entry factor (41/44 against 1/44) must lift
// them to at least 75%, while command, which a profile never observes,
// keeps its smoothing share.
func TestSampleCellsFollowsTheProfilesEntryKinds(t *testing.T) {
	profile := readProfile(t, profileJSON(t, map[string]int{}, map[string]int{},
		map[string]int{"function": 0, "method": 0, "http_handler": 40}, nil))
	cells, err := evalgen.SampleCells(profile, 200, 7)
	if err != nil {
		t.Fatal(err)
	}
	handlers := countWhere(cells, func(c evalgen.Cell) bool { return c.Entry == "http_handler" })
	if handlers < 150 {
		t.Fatalf("HTTP handlers in %d of 200; the profile weights them over uniform's ~50", handlers)
	}
	if handlers != 182 {
		t.Fatalf("HTTP handlers in %d of 200 at seed 7; the draw is deterministic and was 182", handlers)
	}
	handler, _ := evalgen.CellFromIndex(0)
	command := handler
	handler.Entry, command.Entry = "http_handler", "command"
	if !closeTo(profile.Weight(handler), 41*profile.Weight(command)) || profile.Weight(command) <= 0 {
		t.Fatalf("a handler cell weighs %v and a command cell %v; want 41:1, with command still possible", profile.Weight(handler), profile.Weight(command))
	}
}
