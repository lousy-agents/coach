package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestSortSignals_UnknownSeverityIsNotUniversalBottomRank(t *testing.T) {

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

func TestBuild_IncludeResolvedTrueKeepsResolvedInSignals(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "f.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
			{Kind: "mutates_input", Name: "GoneNow", Location: semantics.Location{StartRow: 2}},
		},
	}
	head := &semantics.Result{
		Path:        "f.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 10}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "f.go", Status: "modified", Base: base, Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 2 {
		t.Fatalf("Report.Signals length: got %d, want 2 (resolved signal kept): %+v", len(report.Signals), report.Signals)
	}
	if report.Summary.ResolvedSignals != 1 {
		t.Errorf("Summary.ResolvedSignals: got %d, want 1", report.Summary.ResolvedSignals)
	}
	if report.Summary.ActiveSignals != 2 {
		t.Errorf("Summary.ActiveSignals: got %d, want 2 (unfiltered count since IncludeResolved is true)", report.Summary.ActiveSignals)
	}
}
