package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestSortSignals_TiebreakersInOrder(t *testing.T) {
	tests := []struct {
		name, order   string
		first, second Signal
	}{
		{"severity", "severity descending",
			sortableSignal("a", "r", "f.go", "existing", false, "low", "medium", 0, 0),
			sortableSignal("b", "r", "f.go", "existing", false, "high", "medium", 0, 0)},
		{"confidence", "confidence descending",
			sortableSignal("a", "r", "f.go", "existing", false, "medium", "low", 0, 0),
			sortableSignal("b", "r", "f.go", "existing", false, "medium", "high", 0, 0)},
		{"path", "path ascending",
			sortableSignal("a", "r", "z.go", "existing", false, "medium", "medium", 0, 0),
			sortableSignal("b", "r", "a.go", "existing", false, "medium", "medium", 0, 0)},
		{"start row", "StartRow ascending",
			sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 10, 0),
			sortableSignal("b", "r", "f.go", "existing", false, "medium", "medium", 1, 0)},
		{"start col", "StartCol ascending",
			sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 1, 10),
			sortableSignal("b", "r", "f.go", "existing", false, "medium", "medium", 1, 1)},
		{"rule id", "RuleID ascending",
			sortableSignal("a", "z.rule", "f.go", "existing", false, "medium", "medium", 0, 0),
			sortableSignal("b", "a.rule", "f.go", "existing", false, "medium", "medium", 0, 0)},
		{"id", "ID ascending",
			sortableSignal("z", "r", "f.go", "existing", false, "medium", "medium", 0, 0),
			sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 0, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectSortPutsSecondFirst(t, tt.order, tt.first, tt.second)
		})
	}
}

// expectSortPutsSecondFirst sorts [first, second], which differ only in the
// tiebreaker under test, and expects second to come out ahead.
func expectSortPutsSecondFirst(t *testing.T, order string, first, second Signal) {
	t.Helper()
	signals := []Signal{first, second}
	sortSignals(signals)
	if signals[0].ID != second.ID || signals[1].ID != first.ID {
		t.Errorf("%s: got order %q,%q, want %q,%q", order, signals[0].ID, signals[1].ID, second.ID, first.ID)
	}
}

func TestSortSignals_PriorityGroupsInOrder(t *testing.T) {
	introducedChanged := sortableSignal("1", "r", "f.go", "introduced", true, "medium", "medium", 0, 0)
	existingChanged := sortableSignal("2", "r", "f.go", "existing", true, "medium", "medium", 0, 0)
	introducedUnchanged := sortableSignal("3", "r", "f.go", "introduced", false, "medium", "medium", 0, 0)
	existingUnchanged := sortableSignal("4", "r", "f.go", "existing", false, "medium", "medium", 0, 0)
	resolved := sortableSignal("5", "r", "f.go", "resolved", false, "medium", "medium", 0, 0)
	unknown := sortableSignal("6", "r", "f.go", "unknown", false, "medium", "medium", 0, 0)
	bogus := sortableSignal("7", "r", "f.go", Lifecycle("bogus"), true, "medium", "medium", 0, 0)

	signals := []Signal{bogus, resolved, existingUnchanged, unknown, introducedUnchanged, existingChanged, introducedChanged}
	sortSignals(signals)

	var gotIDs []string
	for _, s := range signals {
		gotIDs = append(gotIDs, s.ID)
	}

	want := []string{"1", "2", "3", "4", "5", "6", "7"}
	if len(gotIDs) != len(want) {
		t.Fatalf("sorted IDs: got %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Errorf("sorted IDs: got %v, want %v", gotIDs, want)
			break
		}
	}
}

func TestSortSignals_UnrecognizedConfidenceDoesNotPanicAndSortsLast(t *testing.T) {
	bogusConfidence := sortableSignal("c", "r", "f.go", "existing", false, "medium", Confidence("bogus"), 0, 0)
	lowConfidence := sortableSignal("d", "r", "f.go", "existing", false, "medium", "low", 0, 0)

	signals2 := []Signal{bogusConfidence, lowConfidence}
	sortSignals(signals2)

	if signals2[0].ID != "d" || signals2[1].ID != "c" {
		t.Errorf("unrecognized Confidence must sort after low: got order %q,%q, want %q,%q", signals2[0].ID, signals2[1].ID, "d", "c")
	}
}

func TestSortSignals_MetricSignalKeepsPathPositionBesideNoMagnitudeSignal(t *testing.T) {
	noMagnitude := sortableSignal("a", "r", "a.go", "existing", false, "medium", "high", 0, 0)
	metric := sortableSignal("b", cognitiveComplexityRule.ruleID, "z.go", "existing", false, "medium", "high", 0, 0)
	metric.Evidence = "cognitive_complexity=-15"

	signals := []Signal{metric, noMagnitude}
	sortSignals(signals)

	if signals[0].ID != "a" || signals[1].ID != "b" {
		t.Errorf("a metric signal must not outrank a no-magnitude signal of another rule: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "a", "b")
	}
}

func sortableSignal(id, ruleID, path string, lifecycle Lifecycle, changed bool, severity Severity, confidence Confidence, startRow, startCol uint) Signal {
	return Signal{
		ID:         id,
		RuleID:     ruleID,
		Path:       path,
		Lifecycle:  lifecycle,
		Changed:    changed,
		Severity:   severity,
		Confidence: confidence,
		Location:   semantics.Location{StartRow: startRow, StartCol: startCol},
	}
}
