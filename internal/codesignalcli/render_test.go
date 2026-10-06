package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestRenderTextSummaryLineScopeDisclosure(t *testing.T) {
	t.Run("discloses the filtered count under production scope", func(t *testing.T) {
		body_renderTest_disclosesTheFilteredCountUnderProductionScope_13(t)
	})

	t.Run("discloses zero filtered without claiming no scope was applied", func(t *testing.T) {
		body_renderTest_disclosesZeroFilteredWithoutClaimingNoScopeWasAp_32(t)
	})

	t.Run("all scope discloses no filtering applied", func(t *testing.T) {
		body_renderTest_allScopeDisclosesNoFilteringApplied_51(t)
	})

	t.Run("unset applied scope keeps original summary line unchanged", func(t *testing.T) {
		body_renderTest_unsetAppliedScopeKeepsOriginalSummaryLineUnchang_70(t)
	})
}

func TestRenderTextSignalsPresentRenderingIsPinnedExactly(t *testing.T) {
	report := &codesignal.Report{
		Summary: codesignal.Summary{FilesAnalyzed: 1, ActiveSignals: 1},
		Signals: []codesignal.Signal{
			{
				RuleID:         "mutation.input_mutation",
				Severity:       codesignal.Severity("medium"),
				Path:           "a.go",
				SourceScope:    "production",
				Location:       semantics.Location{StartRow: 4},
				Lifecycle:      codesignal.Lifecycle("introduced"),
				Changed:        true,
				Evidence:       "func Update mutates input",
				WhyItMatters:   "callers may not expect their argument to be mutated",
				Recommendation: "return a new value instead of mutating input",
			},
		},
		Diagnostics:     []codesignal.Diagnostic{{Path: "b.go", Kind: "empty_content", Message: "empty"}},
		ProjectCoverage: &projectmodel.Coverage{Phase: "partial", Complete: false},
	}

	got := RenderText(report)

	want := "files analyzed: 1, active signals: 1, diagnostics: 1\n" +
		"rule_id: mutation.input_mutation\n" +
		"severity: medium\n" +
		"path: a.go\n" +
		"line: 5\n" +
		"lifecycle: introduced\n" +
		"source_scope: production\n" +
		"changed: true\n" +
		"evidence: func Update mutates input\n" +
		"why it matters: callers may not expect their argument to be mutated\n" +
		"recommendation: return a new value instead of mutating input\n" +
		"\n" +
		"Diagnostics:\n" +
		"path: b.go, kind: empty_content, message: empty\n" +
		"\n" +
		"Project coverage: phase=partial, complete=false\n"

	if got != want {
		t.Errorf("signals-present path changed; a diagnostic and incomplete ProjectCoverage must not alter it.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

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
			got := RenderTextWithOptions(tc.report, RenderOptions{SeeAllCommand: "coach codesignal --baseline"})
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
			got := RenderText(&codesignal.Report{Signals: []codesignal.Signal{signal}, SignalsWithheld: tc.withheld})
			requireContains(t, got, tc.wantLine)
		})
	}

	t.Run("without a caller-supplied command and nothing withheld the line offers no way to see more", func(t *testing.T) {
		report := &codesignal.Report{
			Signals:         []codesignal.Signal{signal},
			SignalsWithheld: &codesignal.SignalsWithheld{Top: 5},
		}
		if got := RenderText(report); strings.Contains(got, "see all") {
			t.Fatalf("unexpected see-all clause in:\n%s", got)
		}
	})

	t.Run("an unnarrowed report never mentions withheld", func(t *testing.T) {
		if got := RenderText(&codesignal.Report{Signals: []codesignal.Signal{signal}}); strings.Contains(got, "withheld") {
			t.Fatalf("unexpected withheld line in:\n%s", got)
		}
	})
}
