package codesignal

import (
	"encoding/json"
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

func mustNarrow(t *testing.T, report *Report, opts NarrowOptions) *Report {
	t.Helper()
	view, err := report.Narrow(opts)
	if err != nil {
		t.Fatalf("Narrow(%+v): %v", opts, err)
	}
	return view
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
			if !reflect.DeepEqual(signalSeverities(got.Signals), tc.wantKept) {
				t.Fatalf("signals = %v, want %v", signalSeverities(got.Signals), tc.wantKept)
			}
			if len(got.ProjectChanges) != len(tc.wantKept) {
				t.Fatalf("project changes = %d, want %d (they follow their mirrored signal)", len(got.ProjectChanges), len(tc.wantKept))
			}
			want := &SignalsWithheld{MinSeverity: tc.floor, BelowMinSeverity: tc.wantWithheld}
			if !reflect.DeepEqual(got.SignalsWithheld, want) {
				t.Fatalf("withheld = %+v, want %+v", got.SignalsWithheld, want)
			}
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
		got := mustNarrow(t, report, NarrowOptions{Top: tc.n})
		if !reflect.DeepEqual(signalSeverities(got.Signals), tc.wantKept) {
			t.Fatalf("top %d: signals = %v, want %v", tc.n, signalSeverities(got.Signals), tc.wantKept)
		}
		if len(got.ProjectChanges) != len(tc.wantKept) {
			t.Fatalf("top %d: project changes = %d, want %d", tc.n, len(got.ProjectChanges), len(tc.wantKept))
		}
		want := &SignalsWithheld{Top: tc.n, BeyondTop: tc.wantBeyondTop}
		if !reflect.DeepEqual(got.SignalsWithheld, want) {
			t.Fatalf("top %d: withheld = %+v, want %+v", tc.n, got.SignalsWithheld, want)
		}
	}
}

func TestNarrowFloorThenCapCountsEachWithheldSignalOnce(t *testing.T) {
	report := reportWithSeverities("2", "high", "high", "high", "low")

	got := mustNarrow(t, report, NarrowOptions{MinSeverity: "high", Top: 2})

	want := &SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 1, Top: 2, BeyondTop: 1}
	if !reflect.DeepEqual(got.SignalsWithheld, want) {
		t.Fatalf("withheld = %+v, want %+v", got.SignalsWithheld, want)
	}
	if len(got.Signals) != 2 || len(got.ProjectChanges) != 2 {
		t.Fatalf("signals = %d, project changes = %d, want 2 each", len(got.Signals), len(got.ProjectChanges))
	}
}

func TestNarrowLeavesTheSourceReportAndFullAnalysisBlocksUntouched(t *testing.T) {
	report := reportWithSeverities("2", "high", "low")

	got := mustNarrow(t, report, NarrowOptions{MinSeverity: "high", Top: 1})

	if len(report.Signals) != 2 || len(report.ProjectChanges) != 2 || report.SignalsWithheld != nil {
		t.Fatalf("source report was mutated: %+v", report)
	}
	if got.Summary != report.Summary {
		t.Fatalf("summary = %+v, want %+v", got.Summary, report.Summary)
	}
	if got.Coverage != report.Coverage {
		t.Fatal("coverage must describe the full analysis")
	}
}

func TestNarrowKeepsAbsentProjectChangesAbsent(t *testing.T) {
	report := reportWithSeverities("2", "high", "low")
	report.ProjectChanges = nil

	got := mustNarrow(t, report, NarrowOptions{MinSeverity: "high"})

	if got.ProjectChanges != nil {
		t.Fatalf("project changes = %+v, want nil", got.ProjectChanges)
	}
}

func TestNarrowTreatsUnknownSeverityLikeLow(t *testing.T) {
	report := reportWithSeverities("1", "bogus")

	if got := mustNarrow(t, report, NarrowOptions{MinSeverity: "low"}); len(got.Signals) != 1 {
		t.Fatalf("unknown severity must survive a low floor, got %d signals", len(got.Signals))
	}
	if got := mustNarrow(t, report, NarrowOptions{MinSeverity: "advisory"}); len(got.Signals) != 0 || got.SignalsWithheld.BelowMinSeverity != 1 {
		t.Fatalf("unknown severity must be withheld above low, got %+v", got)
	}
}

func TestParseSeverityFloor(t *testing.T) {
	for _, valid := range []string{"high", "medium", "advisory", "low"} {
		if got, ok := ParseSeverityFloor(valid); !ok || string(got) != valid {
			t.Fatalf("ParseSeverityFloor(%q) = %q, %t", valid, got, ok)
		}
	}
	for _, invalid := range []string{"", "HIGH", "urgent", "critical", " high"} {
		if _, ok := ParseSeverityFloor(invalid); ok {
			t.Fatalf("ParseSeverityFloor(%q) accepted", invalid)
		}
	}
}

func signalsWithheldWire(t *testing.T, report *Report) (string, bool) {
	t.Helper()
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	raw, present := document["signals_withheld"]
	return string(raw), present
}

func TestSignalsWithheldWireShapePerNarrowing(t *testing.T) {
	for _, schemaVersion := range []string{"1", "2"} {
		report := reportWithSeverities(schemaVersion, "high", "high", "low")
		if _, present := signalsWithheldWire(t, report); present {
			t.Fatalf("schema %s: an unnarrowed report must not emit signals_withheld", schemaVersion)
		}
		cases := map[string]struct {
			opts NarrowOptions
			want string
		}{
			"floor only":     {NarrowOptions{MinSeverity: "high"}, `{"min_severity":"high","below_min_severity":1}`},
			"top only":       {NarrowOptions{Top: 1}, `{"top":1,"beyond_top":2}`},
			"top above size": {NarrowOptions{Top: 9}, `{"top":9,"beyond_top":0}`},
			"floor and top":  {NarrowOptions{MinSeverity: "high", Top: 1}, `{"min_severity":"high","below_min_severity":1,"top":1,"beyond_top":1}`},
			"floor, zero":    {NarrowOptions{MinSeverity: "low"}, `{"min_severity":"low","below_min_severity":0}`},
		}
		for name, tc := range cases {
			t.Run("schema "+schemaVersion+" "+name, func(t *testing.T) {
				got, _ := signalsWithheldWire(t, mustNarrow(t, report, tc.opts))
				if got != tc.want {
					t.Fatalf("signals_withheld = %s, want %s", got, tc.want)
				}
			})
		}
	}
}
