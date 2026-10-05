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

func TestWithMinSeverityKeepsAtOrAboveFloorInOrder(t *testing.T) {
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
			got := report.WithMinSeverity(tc.floor)
			if !reflect.DeepEqual(signalSeverities(got.Signals), tc.wantKept) {
				t.Fatalf("signals = %v, want %v", signalSeverities(got.Signals), tc.wantKept)
			}
			if len(got.ProjectChanges) != len(tc.wantKept) {
				t.Fatalf("project changes = %d, want %d (same rule as signals)", len(got.ProjectChanges), len(tc.wantKept))
			}
			want := &SignalsWithheld{MinSeverity: tc.floor, BelowMinSeverity: tc.wantWithheld}
			if !reflect.DeepEqual(got.SignalsWithheld, want) {
				t.Fatalf("withheld = %+v, want %+v", got.SignalsWithheld, want)
			}
		})
	}
}

func TestWithMinSeverityLeavesTheSourceReportAndFullAnalysisBlocksUntouched(t *testing.T) {
	report := reportWithSeverities("2", "high", "low")

	got := report.WithMinSeverity("high")

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

func TestWithMinSeverityTreatsUnknownSeverityLikeLow(t *testing.T) {
	report := reportWithSeverities("1", "critical")

	if got := report.WithMinSeverity("low"); len(got.Signals) != 1 {
		t.Fatalf("unknown severity must survive a low floor, got %d signals", len(got.Signals))
	}
	if got := report.WithMinSeverity("advisory"); len(got.Signals) != 0 || got.SignalsWithheld.BelowMinSeverity != 1 {
		t.Fatalf("unknown severity must be withheld above low, got %+v", got)
	}
}

func TestSignalsWithheldWireShape(t *testing.T) {
	for _, schemaVersion := range []string{"1", "2"} {
		t.Run("schema "+schemaVersion, func(t *testing.T) {
			report := reportWithSeverities(schemaVersion, "high", "low")

			plain, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			var plainDoc map[string]json.RawMessage
			if err := json.Unmarshal(plain, &plainDoc); err != nil {
				t.Fatal(err)
			}
			if _, present := plainDoc["signals_withheld"]; present {
				t.Fatalf("unnarrowed report must not emit signals_withheld: %s", plain)
			}

			narrowed, err := json.Marshal(report.WithMinSeverity("high"))
			if err != nil {
				t.Fatal(err)
			}
			var narrowedDoc map[string]json.RawMessage
			if err := json.Unmarshal(narrowed, &narrowedDoc); err != nil {
				t.Fatal(err)
			}
			if got, want := string(narrowedDoc["signals_withheld"]), `{"min_severity":"high","below_min_severity":1}`; got != want {
				t.Fatalf("signals_withheld = %s, want %s", got, want)
			}
		})
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
