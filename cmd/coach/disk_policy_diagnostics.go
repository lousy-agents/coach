package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// withDiskSelectionDiagnostics reports what discovery, which reads the revision, cannot know. The canonical path is
// tried first on disk, so the revision's canonical policy is absent whenever disk selects project.json or nothing, and
// its project.json only when disk selects nothing.
func withDiskSelectionDiagnostics(diags []codesignal.Diagnostic, source policyref.Ref, disk projectconfig.DiskSelection) []codesignal.Diagnostic {
	if _, deprecated := codesignal.DiagnosticOfKind(diags, "project_config_deprecated"); !source.Explicit && disk.Path == projectconfig.DefaultPath && !deprecated {
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
