package codesignal

import (
	"testing"
)

func body_sortPart2Test_severity_8(t *testing.T) {
	low := sortableSignal("a", "r", "f.go", "existing", false, "low", "medium", 0, 0)
	high := sortableSignal("b", "r", "f.go", "existing", false, "high", "medium", 0, 0)
	signals := []Signal{low, high}
	sortSignals(signals)
	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("severity descending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}
}

func body_sortPart2Test_confidence_18(t *testing.T) {
	low := sortableSignal("a", "r", "f.go", "existing", false, "medium", "low", 0, 0)
	high := sortableSignal("b", "r", "f.go", "existing", false, "medium", "high", 0, 0)
	signals := []Signal{low, high}
	sortSignals(signals)
	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("confidence descending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}
}

func body_sortPart2Test_path_28(t *testing.T) {
	z := sortableSignal("a", "r", "z.go", "existing", false, "medium", "medium", 0, 0)
	a := sortableSignal("b", "r", "a.go", "existing", false, "medium", "medium", 0, 0)
	signals := []Signal{z, a}
	sortSignals(signals)
	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("path ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}
}

func body_sortPart2Test_startRow_38(t *testing.T) {
	later := sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 10, 0)
	earlier := sortableSignal("b", "r", "f.go", "existing", false, "medium", "medium", 1, 0)
	signals := []Signal{later, earlier}
	sortSignals(signals)
	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("StartRow ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}
}

func body_sortPart2Test_startCol_48(t *testing.T) {
	later := sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 1, 10)
	earlier := sortableSignal("b", "r", "f.go", "existing", false, "medium", "medium", 1, 1)
	signals := []Signal{later, earlier}
	sortSignals(signals)
	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("StartCol ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}
}

func body_sortPart2Test_ruleId_58(t *testing.T) {
	z := sortableSignal("a", "z.rule", "f.go", "existing", false, "medium", "medium", 0, 0)
	a := sortableSignal("b", "a.rule", "f.go", "existing", false, "medium", "medium", 0, 0)
	signals := []Signal{z, a}
	sortSignals(signals)
	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("RuleID ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}
}

func body_sortPart2Test_id_68(t *testing.T) {
	z := sortableSignal("z", "r", "f.go", "existing", false, "medium", "medium", 0, 0)
	a := sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 0, 0)
	signals := []Signal{z, a}
	sortSignals(signals)
	if signals[0].ID != "a" || signals[1].ID != "z" {
		t.Errorf("ID ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "a", "z")
	}
}
