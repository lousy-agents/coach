package main

import (
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

func projectSelectionDiagnostics(selection projectconfig.Selection) []codesignal.Diagnostic {
	var diags []codesignal.Diagnostic
	if selection.Deprecated {
		diags = append(diags, codesignal.Diagnostic{
			Kind:    "project_config_deprecated",
			Path:    projectconfig.DefaultPath,
			Message: deprecatedProjectConfigMessage,
		})
	}
	return diags
}

// withoutOutdatedDeprecation drops the deprecation of the root project.json when the scan used another policy, which
// is the case where the canonical file on disk replaces it.
func withoutOutdatedDeprecation(diags []codesignal.Diagnostic, usedPath string) []codesignal.Diagnostic {
	if usedPath == projectconfig.DefaultPath {
		return diags
	}
	var kept []codesignal.Diagnostic
	for _, diag := range diags {
		if diag.Kind != "project_config_deprecated" {
			kept = append(kept, diag)
		}
	}
	return kept
}
