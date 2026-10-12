package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("missing %q in:\n%s", want, got)
	}
}

func TestRenderTextWithheldSignalsLine(t *testing.T) {
	signal := codesignal.Signal{RuleID: "r", Severity: "high", Path: "a.go"}

	cases := []struct {
		name     string
		report   *codesignal.Report
		wantLine string
		wantLead string
	}{
		{
			name: "baseline plural",
			report: &codesignal.Report{
				Scope:           codesignal.Scope{Baseline: true},
				Signals:         []codesignal.Signal{signal},
				SignalsWithheld: &codesignal.SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 2},
			},
			wantLine: "withheld: 2 signals below --min-severity high; summary counts describe the full analysis; see all: coach codesignal --baseline\n",
		},
		{
			name: "diff singular",
			report: &codesignal.Report{
				Signals:         []codesignal.Signal{signal},
				SignalsWithheld: &codesignal.SignalsWithheld{MinSeverity: "medium", BelowMinSeverity: 1},
			},
			wantLine: "withheld: 1 signal below --min-severity medium; summary counts describe the full analysis; see all: coach codesignal --baseline\n",
		},
		{
			name: "everything withheld scopes the all-clear to the floor",
			report: &codesignal.Report{
				SignalsWithheld: &codesignal.SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 3},
			},
			wantLine: "withheld: 3 signals below --min-severity high",
			wantLead: "No active CodeSignal findings at or above --min-severity high.\n",
		},
		{
			name: "cap only names the command to see all",
			report: &codesignal.Report{
				Signals:         []codesignal.Signal{signal},
				SignalsWithheld: &codesignal.SignalsWithheld{Top: 1, BeyondTop: 3},
			},
			wantLine: "withheld: 3 signals beyond --top 1; summary counts describe the full analysis; see all: coach codesignal --baseline\n",
		},
		{
			name: "cap singular",
			report: &codesignal.Report{
				Signals:         []codesignal.Signal{signal},
				SignalsWithheld: &codesignal.SignalsWithheld{Top: 2, BeyondTop: 1},
			},
			wantLine: "withheld: 1 signal beyond --top 2; summary counts describe the full analysis; see all: coach codesignal --baseline\n",
		},
		{
			name: "cap at or above the signal count offers nothing more to see",
			report: &codesignal.Report{
				Signals:         []codesignal.Signal{signal},
				SignalsWithheld: &codesignal.SignalsWithheld{Top: 5},
			},
			wantLine: "withheld: 0 signals beyond --top 5; summary counts describe the full analysis\n",
		},
		{
			name: "floor and cap report both counts and the total and drop both flags",
			report: &codesignal.Report{
				Scope:           codesignal.Scope{Baseline: true},
				Signals:         []codesignal.Signal{signal},
				SignalsWithheld: &codesignal.SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 2, Top: 1, BeyondTop: 3},
			},
			wantLine: "withheld: 5 signals (2 below --min-severity high, 3 beyond --top 1); summary counts describe the full analysis; see all: coach codesignal --baseline\n",
		},
		{
			name: "zero withheld keeps the unscoped all-clear",
			report: &codesignal.Report{
				SignalsWithheld: &codesignal.SignalsWithheld{MinSeverity: "low"},
			},
			wantLine: "withheld: 0 signals below --min-severity low",
			wantLead: "No active CodeSignal findings.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReportTextWithOptions(tc.report, TextOptions{SeeAllCommand: "coach codesignal --baseline"})
			requireContains(t, got, tc.wantLine)
			requireContains(t, got, tc.wantLead)
		})
	}

	fallbackCases := []struct {
		name     string
		withheld *codesignal.SignalsWithheld
		wantLine string
	}{
		{
			name:     "cap alone",
			withheld: &codesignal.SignalsWithheld{Top: 1, BeyondTop: 3},
			wantLine: "withheld: 3 signals beyond --top 1; summary counts describe the full analysis; see all: re-run without --top\n",
		},
		{
			name:     "floor alone",
			withheld: &codesignal.SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 2},
			wantLine: "withheld: 2 signals below --min-severity high; summary counts describe the full analysis; see all: re-run without --min-severity\n",
		},
		{
			name:     "floor and cap",
			withheld: &codesignal.SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 2, Top: 1, BeyondTop: 3},
			wantLine: "withheld: 5 signals (2 below --min-severity high, 3 beyond --top 1); summary counts describe the full analysis; see all: re-run without --min-severity and --top\n",
		},
		{
			name:     "floor and cap where only the floor withheld anything",
			withheld: &codesignal.SignalsWithheld{MinSeverity: "high", BelowMinSeverity: 2, Top: 5},
			wantLine: "withheld: 2 signals (2 below --min-severity high, 0 beyond --top 5); summary counts describe the full analysis; see all: re-run without --min-severity and --top\n",
		},
	}
	for _, tc := range fallbackCases {
		t.Run("without a caller-supplied command, "+tc.name+" names only the flags in effect", func(t *testing.T) {
			got := ReportText(&codesignal.Report{Signals: []codesignal.Signal{signal}, SignalsWithheld: tc.withheld})
			requireContains(t, got, tc.wantLine)
		})
	}

	t.Run("without a caller-supplied command and nothing withheld the line offers no way to see more", func(t *testing.T) {
		report := &codesignal.Report{
			Signals:         []codesignal.Signal{signal},
			SignalsWithheld: &codesignal.SignalsWithheld{Top: 5},
		}
		if got := ReportText(report); strings.Contains(got, "see all") {
			t.Fatalf("unexpected see-all clause in:\n%s", got)
		}
	})

	t.Run("an unnarrowed report never mentions withheld", func(t *testing.T) {
		if got := ReportText(&codesignal.Report{Signals: []codesignal.Signal{signal}}); strings.Contains(got, "withheld") {
			t.Fatalf("unexpected withheld line in:\n%s", got)
		}
	})
}
