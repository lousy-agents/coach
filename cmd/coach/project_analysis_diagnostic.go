package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
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

// withDiskSelectionDiagnostics adds what the policy on disk says that discovery, which reads the revision, did not: a
// root project.json on disk is the deprecated fallback even where the revision selects the canonical policy, and the
// policy the revision selects is reported where nothing is on disk at it. The canonical path is tried first, so it is
// absent whenever the working tree selects the root project.json or none; the root project.json is absent only when
// the working tree selects none.
func withDiskSelectionDiagnostics(diags []codesignal.Diagnostic, source policyref.Ref, disk projectconfig.DiskSelection) []codesignal.Diagnostic {
	if !source.Explicit && disk.Path == projectconfig.DefaultPath && !hasDiagnosticKind(diags, "project_config_deprecated") {
		diags = appendDiagnostic(diags, codesignal.Diagnostic{
			Kind:    "project_config_deprecated",
			Path:    projectconfig.DefaultPath,
			Message: deprecatedProjectConfigMessage,
		})
	}
	headSelectionAbsent := disk.Path == "" || (source.Path == projectconfig.CanonicalPath && disk.Path == projectconfig.DefaultPath)
	if source.Path == "" || !headSelectionAbsent {
		return diags
	}
	return appendDiagnostic(diags, codesignal.Diagnostic{
		Kind:    codesignal.DiagKindProjectPolicyAbsentOnDisk,
		Path:    source.Path,
		Message: "The project policy HEAD selects is not on disk, so this scan did not read it.",
	})
}

// absentPolicyPath is the policy the revision selects that the working tree does not hold, empty where there is none.
// A worktree scan counts git hiding that path as in-scope.
func absentPolicyPath(diags []codesignal.Diagnostic) string {
	for _, diag := range diags {
		if diag.Kind == codesignal.DiagKindProjectPolicyAbsentOnDisk {
			return diag.Path
		}
	}
	return ""
}

func hasDiagnosticKind(diags []codesignal.Diagnostic, kind string) bool {
	for _, diag := range diags {
		if diag.Kind == kind {
			return true
		}
	}
	return false
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
