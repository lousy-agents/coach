package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// withDiskSelectionDiagnostics adds what the policy on disk says that discovery, which reads the revision, did not: a
// root project.json on disk is the deprecated fallback even where the revision selects the canonical policy, and the
// policy the revision selects is reported where nothing is on disk at it. The canonical path is tried first, so it is
// absent whenever the working tree selects the root project.json or none; the root project.json is absent only when
// the working tree selects none.
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
