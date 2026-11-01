package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// verdictLineAfterLayerMembership returns the line printed directly after the
// layer-membership line, which is where the verdict sits.
func verdictLineAfterLayerMembership(t *testing.T, text string) string {
	t.Helper()
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "sit outside every declared layer") && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	t.Fatalf("no layer-membership line in:\n%s", text)
	return ""
}

// TestEveryNoActiveFindingsVerdictCarriesTheLayerReach lists one report per
// verdict form. A new verdict branch needs a row here, and the row fails until
// that branch states the reach of forbidden_imports.
func TestEveryNoActiveFindingsVerdictCarriesTheLayerReach(t *testing.T) {
	unresolved := []codesignal.Diagnostic{{Path: "src/a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "m"}}
	noMatch := []codesignal.ProjectNextAction{{Kind: "review_policy_coverage"}}
	tests := []struct {
		name        string
		report      codesignal.Report
		verdictLead string
	}{
		{
			name:        "all-clear",
			report:      codesignal.Report{},
			verdictLead: "No active CodeSignal findings;",
		},
		{
			name:        "all-clear below a severity floor",
			report:      codesignal.Report{SignalsWithheld: &codesignal.SignalsWithheld{MinSeverity: "medium", BelowMinSeverity: 1}},
			verdictLead: "No active CodeSignal findings at or above --min-severity medium;",
		},
		{
			name:        "complete no-match scan",
			report:      codesignal.Report{ProjectNextActions: noMatch},
			verdictLead: "Complete scan found no configured covered match;",
		},
		{
			name:        "complete no-match scan with unanalyzed paths",
			report:      codesignal.Report{ProjectNextActions: noMatch, Summary: codesignal.Summary{FilesUnanalyzed: 2}},
			verdictLead: "Complete scan found no configured covered match, but",
		},
		{
			name:        "no-match scan with unresolved imports",
			report:      codesignal.Report{ProjectNextActions: noMatch, Diagnostics: unresolved},
			verdictLead: "Scan found no configured covered match, but the analysis is incomplete:",
		},
		{
			name:        "no-match scan with unresolved imports and unanalyzed paths",
			report:      codesignal.Report{ProjectNextActions: noMatch, Diagnostics: unresolved, Summary: codesignal.Summary{FilesUnanalyzed: 2}},
			verdictLead: "Scan found no configured covered match, but the analysis is incomplete:",
		},
		{
			name:        "incomplete project analysis",
			report:      codesignal.Report{ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: false}},
			verdictLead: "No active CodeSignal findings, but the analysis is incomplete:",
		},
		{
			name:        "unresolved imports without a no-match action",
			report:      codesignal.Report{Diagnostics: unresolved},
			verdictLead: "No active CodeSignal findings, but the analysis is incomplete:",
		},
		{
			name:        "unclassified diagnostics",
			report:      codesignal.Report{Diagnostics: []codesignal.Diagnostic{{Path: "x", Kind: "some_new_kind", Message: "m"}}},
			verdictLead: "No active CodeSignal findings, but the analysis is incomplete: additional diagnostics were recorded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := tt.report
			report.LayerMembership = layerMembershipWith(3)

			verdict := verdictLineAfterLayerMembership(t, ReportText(&report))

			if !strings.HasPrefix(verdict, tt.verdictLead) {
				t.Fatalf("verdict = %q, want it to start with %q", verdict, tt.verdictLead)
			}
			if !strings.HasSuffix(verdict, layerReachClause+".") {
				t.Fatalf("verdict = %q, want it to end with the reach of forbidden_imports %q", verdict, layerReachClause)
			}
		})
	}
}
