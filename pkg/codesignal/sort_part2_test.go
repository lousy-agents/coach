package codesignal

import (
	"testing"
)

func TestSortSignals_TiebreakersInOrder(t *testing.T) {
	t.Run("severity", func(t *testing.T) {
		body_sortPart2Test_severity_8(t)
	})

	t.Run("confidence", func(t *testing.T) {
		body_sortPart2Test_confidence_18(t)
	})

	t.Run("path", func(t *testing.T) {
		body_sortPart2Test_path_28(t)
	})

	t.Run("start row", func(t *testing.T) {
		body_sortPart2Test_startRow_38(t)
	})

	t.Run("start col", func(t *testing.T) {
		body_sortPart2Test_startCol_48(t)
	})

	t.Run("rule id", func(t *testing.T) {
		body_sortPart2Test_ruleId_58(t)
	})

	t.Run("id", func(t *testing.T) {
		body_sortPart2Test_id_68(t)
	})
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
