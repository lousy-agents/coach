package codesignal

import (
	"testing"
)

func TestSortSignals_AdvisorySeverityRanksAboveLow(t *testing.T) {
	if got, low := severityRank("advisory"), severityRank("low"); got <= low {
		t.Fatalf("severityRank(%q) = %d, must be strictly greater than severityRank(%q) = %d (issue #259)", "advisory", got, "low", low)
	}

	advisory := sortableSignal("a", "r", "f.go", "existing", false, "advisory", "medium", 0, 0)
	low := sortableSignal("b", "r", "f.go", "existing", false, "low", "medium", 0, 0)

	signals := []Signal{low, advisory}
	sortSignals(signals)

	if signals[0].ID != "a" || signals[1].ID != "b" {
		t.Errorf("advisory severity must sort ahead of low: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "a", "b")
	}
}

func TestSortSignals_UnknownSeverityIsNotUniversalBottomRank(t *testing.T) {
	// known is high/medium/low: the property under test is that an unknown
	// severity ties with "low" (rank 1) rather than sinking below every known
	// severity. advisory's own rank is asserted separately in
	// TestSortSignals_AdvisorySeverityRanksAboveLow.
	known := []Severity{"high", "medium", "low"}
	unknownRank := severityRank(Severity("bogus"))

	belowAll := true
	for _, s := range known {
		if unknownRank >= severityRank(s) {
			belowAll = false
			break
		}
	}
	if belowAll {
		ranks := make(map[Severity]int, len(known))
		for _, s := range known {
			ranks[s] = severityRank(s)
		}
		t.Fatalf("severityRank(%q) = %d must not be strictly below every known severity's rank (known ranks: %+v) -- issue #259 latent trap", "bogus", unknownRank, ranks)
	}

	// An unrecognized Severity must not panic sortSignals, and since it ties
	// with "low" at rank 1, ordering must fall through to the ID tiebreak
	// deterministically regardless of input order (issue #259).
	bogus := sortableSignal("a", "r", "f.go", "existing", false, Severity("bogus"), "medium", 0, 0)
	low := sortableSignal("b", "r", "f.go", "existing", false, "low", "medium", 0, 0)

	forward := []Signal{low, bogus}
	sortSignals(forward)
	if forward[0].ID != "a" || forward[1].ID != "b" {
		t.Errorf("unrecognized severity vs low (input low,bogus): got order %q,%q, want %q,%q", forward[0].ID, forward[1].ID, "a", "b")
	}

	reversed := []Signal{bogus, low}
	sortSignals(reversed)
	if reversed[0].ID != "a" || reversed[1].ID != "b" {
		t.Errorf("unrecognized severity vs low (input bogus,low): got order %q,%q, want %q,%q", reversed[0].ID, reversed[1].ID, "a", "b")
	}
}
