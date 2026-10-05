package codesignalcli

import (
	"fmt"

	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// RenderText renders report as deterministic, ANSI-free plain text.
func RenderText(report *codesignal.Report) string {
	var b strings.Builder
	renderReportSummary(&b, report)
	renderProjectScopeSection(&b, report.ProjectScope)
	renderActiveFindings(&b, report)
	renderProjectFacts(&b, report.ProjectFacts)
	renderDiagnosticsSection(&b, report.Diagnostics)
	renderCoverageSection(&b, report.Coverage)
	renderProjectCoverageSection(&b, report.ProjectCoverage)
	renderProjectProvenanceSection(&b, report.ProjectProvenance)
	renderProjectNextActionsSection(&b, report.ProjectNextActions)
	return b.String()
}

func renderActiveFindings(b *strings.Builder, report *codesignal.Report) {
	if len(report.Signals) == 0 && len(report.ProjectChanges) == 0 {
		renderNoActiveFindingsVerdict(b, report)
		renderProjectSummary(b, report.ProjectSummary)
		return
	}

	renderedSignal := renderFileLocalSignals(b, report.Signals, projectChangeSignalIDs(report.ProjectChanges))
	if len(report.ProjectChanges) > 0 {
		if renderedSignal {
			b.WriteString("\n")
		}
		renderProjectChanges(b, report)
		return
	}
	if report.ProjectSummary != nil {
		renderProjectSummary(b, report.ProjectSummary)
	}
}

// renderNoActiveFindingsVerdict qualifies the verdict from a diagnostic Kind,
// not only report.ProjectCoverage.Complete: Report exposes only head-side
// coverage, and a diff's base side can be incomplete while the head side is
// complete, which the Complete field alone cannot see.
func renderNoActiveFindingsVerdict(b *strings.Builder, report *codesignal.Report) {
	if len(report.ProjectNextActions) > 0 {
		if report.Summary.FilesUnanalyzed == 0 {
			b.WriteString("Complete scan found no configured covered match.\n")
			return
		}
		fmt.Fprintf(b, "Complete scan found no configured covered match, but %s.\n", pathCountClause(report.Summary.FilesUnanalyzed))
		return
	}

	incompleteProject := (report.ProjectCoverage != nil && !report.ProjectCoverage.Complete) ||
		hasProjectLifecycleDiagnostic(report.Diagnostics)
	if keepUnqualifiedAllClear(report, incompleteProject) {
		fmt.Fprintf(b, "%s.\n", noActiveFindingsLead(report))
		return
	}

	var causes []string
	if n := report.Summary.FilesUnanalyzed; n > 0 {
		causes = append(causes, pathCountClause(n))
	}
	if incompleteProject {
		causes = append(causes, "project analysis did not complete")
	}
	if len(causes) == 0 {
		causes = append(causes, "additional diagnostics were recorded")
	}
	fmt.Fprintf(b, "%s, but the analysis is incomplete: %s.\n", noActiveFindingsLead(report), strings.Join(causes, "; "))
}

// noActiveFindingsLead scopes the all-clear to the severity floor: when a
// floor withheld signals, "no active findings" would claim more than the
// narrowed view can show.
func noActiveFindingsLead(report *codesignal.Report) string {
	if withheld := report.SignalsWithheld; withheld != nil && withheld.BelowMinSeverity > 0 {
		return "No active CodeSignal findings at or above --min-severity " + string(withheld.MinSeverity)
	}
	return "No active CodeSignal findings"
}

func renderSignal(b *strings.Builder, signal codesignal.Signal) {
	fmt.Fprintf(b, "rule_id: %s\n", signal.RuleID)
	fmt.Fprintf(b, "severity: %s\n", signal.Severity)
	fmt.Fprintf(b, "path: %s\n", signal.Path)
	fmt.Fprintf(b, "line: %d\n", signal.Location.StartRow+1)
	fmt.Fprintf(b, "lifecycle: %s\n", signal.Lifecycle)
	fmt.Fprintf(b, "source_scope: %s\n", signal.SourceScope)
	fmt.Fprintf(b, "changed: %t\n", signal.Changed)
	fmt.Fprintf(b, "evidence: %s\n", signal.Evidence)
	fmt.Fprintf(b, "why it matters: %s\n", signal.WhyItMatters)
	fmt.Fprintf(b, "recommendation: %s\n", signal.Recommendation)
}

func renderDiagnostic(b *strings.Builder, diagnostic codesignal.Diagnostic) {
	fmt.Fprintf(b, "path: %s, kind: %s, message: %s\n", diagnostic.Path, diagnostic.Kind, diagnostic.Message)
}

// RenderJSON renders report as its canonical JSON representation followed
// by exactly one trailing newline, with no CLI-only wrapper fields added.
