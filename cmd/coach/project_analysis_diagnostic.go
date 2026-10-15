package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

const deprecatedProjectConfigMessage = "project.json at the repository root is deprecated. Coach will keep reading it until you move it to .lousy-agents/coach.json."

func withProjectDiagnostic(report *codesignal.Report, diag *codesignal.Diagnostic, extra ...codesignal.Diagnostic) *codesignal.Report {
	if report == nil {
		return report
	}
	combined := append([]codesignal.Diagnostic{}, extra...)
	if diag != nil {
		combined = append(combined, *diag)
	}
	if len(combined) == 0 {
		return report
	}
	out := *report
	out.Diagnostics = append(append([]codesignal.Diagnostic(nil), report.Diagnostics...), combined...)
	codesignal.SortDiagnostics(out.Diagnostics)
	return &out
}

func projectSelectionDiagnostics(selection projectconfig.Selection, revision string, suppressUncommittedCanonical bool) []codesignal.Diagnostic {
	var diags []codesignal.Diagnostic
	if selection.Deprecated {
		diags = append(diags, codesignal.Diagnostic{
			Kind:    "project_config_deprecated",
			Path:    projectconfig.DefaultPath,
			Message: deprecatedProjectConfigMessage,
		})
	}
	// suppressUncommittedCanonical is set only after this invocation created
	// .lousy-agents/coach.json. Discover runs again inside prepareProjectAnalysis
	// and would otherwise report that new worktree file.
	if selection.UncommittedCanonical && !suppressUncommittedCanonical {
		diags = append(diags, codesignal.Diagnostic{
			Kind:    "project_config_uncommitted",
			Path:    projectconfig.CanonicalPath,
			Message: fmt.Sprintf(".lousy-agents/coach.json is in the worktree but is not committed at %s, so this scan did not read it.", revision),
		})
	}
	return diags
}
