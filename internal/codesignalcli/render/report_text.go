// Package render presents CodeSignal reports and project-readiness results as
// the CLI's text and JSON output.
package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// TextOptions carries what only the caller knows about how a report was
// produced.
type TextOptions struct {
	// SeeAllCommand is the invocation that shows every signal a narrowed
	// report withheld. It is text-only presentation and never enters the JSON
	// report. Empty means that, when something was withheld, the line says to
	// re-run without the narrowing flags in effect instead of naming a command.
	SeeAllCommand string
}

// ReportText renders report as deterministic, ANSI-free plain text.
func ReportText(report *codesignal.Report) string {
	return ReportTextWithOptions(report, TextOptions{})
}

// ReportTextWithOptions renders like ReportText. SeeAllCommand is printed
// verbatim into the report, so the caller shall have already made it safe for a
// terminal.
func ReportTextWithOptions(report *codesignal.Report, opts TextOptions) string {
	var b strings.Builder
	renderReportSummary(&b, report)
	renderWithheldSignals(&b, report.SignalsWithheld, opts.SeeAllCommand)
	renderProjectScopeSection(&b, report.ProjectScope)
	renderLayerMembershipLine(&b, report.LayerMembership, report.Scope.Worktree != nil)
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

func pathCountClause(n int) string {
	if n == 1 {
		return "1 path was not analyzed"
	}
	return fmt.Sprintf("%d paths were not analyzed", n)
}
