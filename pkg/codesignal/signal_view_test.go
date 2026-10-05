package codesignal

import (
	"fmt"
	"reflect"
	"testing"
)

func reportWithSeverities(schemaVersion string, severities ...Severity) *Report {
	report := &Report{
		SchemaVersion: schemaVersion,
		Summary:       Summary{ActiveSignals: len(severities)},
		Coverage:      &Coverage{TrackedFilesDiscovered: 7},
	}
	for i, severity := range severities {
		id := string(rune('a' + i))
		report.Signals = append(report.Signals, Signal{ID: id, Severity: severity})
		report.ProjectChanges = append(report.ProjectChanges, ProjectChange{ID: id, Severity: severity})
	}
	return report
}

func signalSeverities(signals []Signal) []Severity {
	out := make([]Severity, 0, len(signals))
	for _, signal := range signals {
		out = append(out, signal.Severity)
	}
	return out
}

func mustNarrow(t *testing.T, report *Report, opts NarrowOptions) Report {
	t.Helper()
	view, err := report.Narrow(opts)
	if err != nil {
		t.Fatalf("Narrow(%+v): %v", opts, err)
	}
	return *view
}

func assertNarrowedView(t *testing.T, got Report, wantKept []Severity, wantWithheld SignalsWithheld) {
	t.Helper()
	if !reflect.DeepEqual(signalSeverities(got.Signals), wantKept) {
		t.Fatalf("signals = %v, want %v", signalSeverities(got.Signals), wantKept)
	}
	if len(got.ProjectChanges) != len(wantKept) {
		t.Fatalf("project changes = %d, want %d (they follow their mirrored signal)", len(got.ProjectChanges), len(wantKept))
	}
	if !reflect.DeepEqual(got.SignalsWithheld, &wantWithheld) {
		t.Fatalf("withheld = %+v, want %+v", got.SignalsWithheld, &wantWithheld)
	}
}

func TestNarrowFloorKeepsAtOrAboveFloorInOrder(t *testing.T) {
	report := reportWithSeverities("2", "high", "medium", "advisory", "low", "high")

	cases := []struct {
		floor        Severity
		wantKept     []Severity
		wantWithheld int
	}{
		{"high", []Severity{"high", "high"}, 3},
		{"medium", []Severity{"high", "medium", "high"}, 2},
		{"advisory", []Severity{"high", "medium", "advisory", "high"}, 1},
		{"low", []Severity{"high", "medium", "advisory", "low", "high"}, 0},
	}
	for _, tc := range cases {
		t.Run(string(tc.floor), func(t *testing.T) {
			got := mustNarrow(t, report, NarrowOptions{MinSeverity: tc.floor})
			assertNarrowedView(t, got, tc.wantKept, SignalsWithheld{MinSeverity: tc.floor, BelowMinSeverity: tc.wantWithheld})
		})
	}
}

func TestNarrowCapKeepsTheLeadingSignalsAndTheirProjectChanges(t *testing.T) {
	report := reportWithSeverities("2", "high", "medium", "advisory", "low")

	cases := []struct {
		n             int
		wantKept      []Severity
		wantBeyondTop int
	}{
		{1, []Severity{"high"}, 3},
		{3, []Severity{"high", "medium", "advisory"}, 1},
		{4, []Severity{"high", "medium", "advisory", "low"}, 0},
		{99, []Severity{"high", "medium", "advisory", "low"}, 0},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("top %d", tc.n), func(t *testing.T) {
			got := mustNarrow(t, report, NarrowOptions{Top: tc.n})
			assertNarrowedView(t, got, tc.wantKept, SignalsWithheld{Top: tc.n, BeyondTop: tc.wantBeyondTop})
		})
	}
}

func TestNarrowFloorThenCapCountsEachWithheldSignalOnce(t *testing.T) {
	report := reportWithSeverities("2", "high", "high", "high", "low")

	got := mustNarrow(t, report, NarrowOptions{MinSeverity: "high", Top: 2})

	assertNarrowedView(t, got, []Severity{"high", "high"},
		SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 1, Top: 2, BeyondTop: 1})
}
