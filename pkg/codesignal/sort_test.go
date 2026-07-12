package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestChangedRangeOverlap_ValidateChangedRangesSplitsInvalidFromValid(t *testing.T) {
	fc := FileChange{
		Path: "f.go",
		ChangedRanges: []LineRange{
			{StartRow: 1, EndRow: 3},
			{StartRow: 5, EndRow: 2},
			{StartRow: 10, EndRow: 10},
			{StartRow: 8, EndRow: 4},
		},
	}

	diagnostics, valid := validateChangedRanges(fc)

	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics length: got %d, want 2: %+v", len(diagnostics), diagnostics)
	}
	for _, d := range diagnostics {
		if d.Path != "f.go" {
			t.Errorf("Diagnostic.Path: got %q, want %q", d.Path, "f.go")
		}
		if d.Kind != "invalid_changed_range" {
			t.Errorf("Diagnostic.Kind: got %q, want %q", d.Kind, "invalid_changed_range")
		}
		if d.Message == "" {
			t.Errorf("Diagnostic.Message must not be empty: %+v", d)
		}
	}

	wantValid := []LineRange{
		{StartRow: 1, EndRow: 3},
		{StartRow: 10, EndRow: 10},
	}
	if len(valid) != len(wantValid) {
		t.Fatalf("valid length: got %d, want %d: %+v", len(valid), len(wantValid), valid)
	}
	for i, r := range wantValid {
		if valid[i] != r {
			t.Errorf("valid[%d]: got %+v, want %+v", i, valid[i], r)
		}
	}
}

func TestChangedRangeOverlap_OverlapsAny(t *testing.T) {
	ranges := []LineRange{{StartRow: 10, EndRow: 20}}

	tests := []struct {
		name string
		loc  semantics.Location
		want bool
	}{
		{"strictly inside", semantics.Location{StartRow: 12, EndRow: 15}, true},
		{"strictly outside before", semantics.Location{StartRow: 1, EndRow: 5}, false},
		{"strictly outside after", semantics.Location{StartRow: 25, EndRow: 30}, false},
		{"loc end row equals range start row", semantics.Location{StartRow: 5, EndRow: 10}, true},
		{"loc start row equals range end row", semantics.Location{StartRow: 20, EndRow: 25}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := overlapsAny(tc.loc, ranges); got != tc.want {
				t.Errorf("overlapsAny(%+v, %+v): got %v, want %v", tc.loc, ranges, got, tc.want)
			}
		})
	}
}

func TestChangedRangeOverlap_MarkChangedNeverMarksResolvedSignals(t *testing.T) {
	ranges := []LineRange{{StartRow: 0, EndRow: 100}}
	signals := []Signal{
		{Lifecycle: "resolved", Location: semantics.Location{StartRow: 5, EndRow: 5}},
		{Lifecycle: "introduced", Location: semantics.Location{StartRow: 5, EndRow: 5}},
	}

	markChanged(signals, ranges)

	if signals[0].Changed {
		t.Errorf("resolved signal must never have Changed=true: %+v", signals[0])
	}
	if !signals[1].Changed {
		t.Errorf("introduced signal overlapping a valid range must have Changed=true: %+v", signals[1])
	}
}

func TestChangedRangeOverlap_MarkChangedOutsideAllRanges(t *testing.T) {
	ranges := []LineRange{{StartRow: 10, EndRow: 20}}
	signals := []Signal{
		{Lifecycle: "existing", Location: semantics.Location{StartRow: 1, EndRow: 1}},
	}

	markChanged(signals, ranges)

	if signals[0].Changed {
		t.Errorf("signal outside all ranges must have Changed=false: %+v", signals[0])
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

func TestSortSignals_TiebreakersInOrder(t *testing.T) {
	t.Run("severity", func(t *testing.T) {
		low := sortableSignal("a", "r", "f.go", "existing", false, "low", "medium", 0, 0)
		high := sortableSignal("b", "r", "f.go", "existing", false, "high", "medium", 0, 0)
		signals := []Signal{low, high}
		sortSignals(signals)
		if signals[0].ID != "b" || signals[1].ID != "a" {
			t.Errorf("severity descending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
		}
	})

	t.Run("confidence", func(t *testing.T) {
		low := sortableSignal("a", "r", "f.go", "existing", false, "medium", "low", 0, 0)
		high := sortableSignal("b", "r", "f.go", "existing", false, "medium", "high", 0, 0)
		signals := []Signal{low, high}
		sortSignals(signals)
		if signals[0].ID != "b" || signals[1].ID != "a" {
			t.Errorf("confidence descending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
		}
	})

	t.Run("path", func(t *testing.T) {
		z := sortableSignal("a", "r", "z.go", "existing", false, "medium", "medium", 0, 0)
		a := sortableSignal("b", "r", "a.go", "existing", false, "medium", "medium", 0, 0)
		signals := []Signal{z, a}
		sortSignals(signals)
		if signals[0].ID != "b" || signals[1].ID != "a" {
			t.Errorf("path ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
		}
	})

	t.Run("start row", func(t *testing.T) {
		later := sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 10, 0)
		earlier := sortableSignal("b", "r", "f.go", "existing", false, "medium", "medium", 1, 0)
		signals := []Signal{later, earlier}
		sortSignals(signals)
		if signals[0].ID != "b" || signals[1].ID != "a" {
			t.Errorf("StartRow ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
		}
	})

	t.Run("start col", func(t *testing.T) {
		later := sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 1, 10)
		earlier := sortableSignal("b", "r", "f.go", "existing", false, "medium", "medium", 1, 1)
		signals := []Signal{later, earlier}
		sortSignals(signals)
		if signals[0].ID != "b" || signals[1].ID != "a" {
			t.Errorf("StartCol ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
		}
	})

	t.Run("rule id", func(t *testing.T) {
		z := sortableSignal("a", "z.rule", "f.go", "existing", false, "medium", "medium", 0, 0)
		a := sortableSignal("b", "a.rule", "f.go", "existing", false, "medium", "medium", 0, 0)
		signals := []Signal{z, a}
		sortSignals(signals)
		if signals[0].ID != "b" || signals[1].ID != "a" {
			t.Errorf("RuleID ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
		}
	})

	t.Run("id", func(t *testing.T) {
		z := sortableSignal("z", "r", "f.go", "existing", false, "medium", "medium", 0, 0)
		a := sortableSignal("a", "r", "f.go", "existing", false, "medium", "medium", 0, 0)
		signals := []Signal{z, a}
		sortSignals(signals)
		if signals[0].ID != "a" || signals[1].ID != "z" {
			t.Errorf("ID ascending: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "a", "z")
		}
	})
}

func TestSortSignals_UnrecognizedSeverityAndConfidenceDoNotPanicAndSortLast(t *testing.T) {
	bogusSeverity := sortableSignal("a", "r", "f.go", "existing", false, Severity("bogus"), "medium", 0, 0)
	low := sortableSignal("b", "r", "f.go", "existing", false, "low", "medium", 0, 0)

	signals := []Signal{bogusSeverity, low}
	sortSignals(signals)

	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("unrecognized Severity must sort after low: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}

	bogusConfidence := sortableSignal("c", "r", "f.go", "existing", false, "medium", Confidence("bogus"), 0, 0)
	lowConfidence := sortableSignal("d", "r", "f.go", "existing", false, "medium", "low", 0, 0)

	signals2 := []Signal{bogusConfidence, lowConfidence}
	sortSignals(signals2)

	if signals2[0].ID != "d" || signals2[1].ID != "c" {
		t.Errorf("unrecognized Confidence must sort after low: got order %q,%q, want %q,%q", signals2[0].ID, signals2[1].ID, "d", "c")
	}
}

func TestBuild_IncludeResolvedFalseHidesResolvedButCountsThem(t *testing.T) {
	b, err := New(Options{})
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

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1 (resolved signal hidden): %+v", len(report.Signals), report.Signals)
	}
	for _, s := range report.Signals {
		if s.Lifecycle == "resolved" {
			t.Errorf("Report.Signals must not contain resolved signals when IncludeResolved is false: %+v", s)
		}
	}
	if report.Summary.ResolvedSignals != 1 {
		t.Errorf("Summary.ResolvedSignals: got %d, want 1", report.Summary.ResolvedSignals)
	}
	if report.Summary.ActiveSignals != len(report.Signals) {
		t.Errorf("Summary.ActiveSignals: got %d, want %d (== len(Report.Signals))", report.Summary.ActiveSignals, len(report.Signals))
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

func TestBuild_ChangedSignalsSortBeforeUnchangedWithinSameLifecycleGroup(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "f.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Unchanged", Location: semantics.Location{StartRow: 100}},
			{Kind: "mutates_input", Name: "Changed", Location: semantics.Location{StartRow: 5}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{
				Path:          "f.go",
				Status:        "added",
				Head:          head,
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 10}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 2 {
		t.Fatalf("Report.Signals length: got %d, want 2: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Subject != "Changed" || !report.Signals[0].Changed {
		t.Errorf("Signals[0] must be the Changed=true signal: %+v", report.Signals[0])
	}
	if report.Signals[1].Subject != "Unchanged" || report.Signals[1].Changed {
		t.Errorf("Signals[1] must be the Changed=false signal: %+v", report.Signals[1])
	}
}
