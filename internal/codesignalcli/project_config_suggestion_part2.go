package codesignalcli

import (
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// suggestPrimaryRootDiagnostic maps result's root-discovery diagnostics to
// the single primary suggestion diagnostic per issue #220's fixed priority:
// unavailable > (outside_snapshot|invalid|duplicate|ambiguous) > incomplete
// > no-modules. ok is true only when result represents a usable, complete,
// non-empty root set.
func suggestPrimaryRootDiagnostic(result projectmodel.RootDiscoveryResult) (code, path, message string, ok bool) {
	for _, diag := range result.Coverage.Diagnostics {
		if diag.Code == projectmodel.DiagRootUnavailable {
			return SuggestDiagSnapshotUnavailable, diag.Path, suggestDiagnosticMessage(diag), false
		}
	}
	for _, diag := range result.Coverage.Diagnostics {
		switch diag.Code {
		case projectmodel.DiagRootOutsideSnapshot, projectmodel.DiagRootInvalid, projectmodel.DiagRootDuplicate, projectmodel.DiagRootAmbiguous:
			return SuggestDiagAmbiguousRoots, diag.Path, suggestDiagnosticMessage(diag), false
		}
	}
	if !result.Complete {
		if code, path, message, found := incompleteRootDiagnostic(result.Coverage.Diagnostics); found {
			return code, path, message, false
		}
		return SuggestDiagIncomplete, "", "coach codesignal --suggest-project-config: Go root discovery did not complete within its resource budget", false
	}
	if len(result.Roots) == 0 {
		return SuggestDiagNoGoModules, "", "coach codesignal --suggest-project-config: no Go module or workspace root was found at HEAD", false
	}
	return "", "", "", true
}

func incompleteRootDiagnostic(diags []projectmodel.Diagnostic) (code, path, message string, found bool) {
	for _, diag := range diags {
		if diag.Code == projectmodel.DiagRootIncomplete {
			return SuggestDiagIncomplete, diag.Path, suggestDiagnosticMessage(diag), true
		}
	}
	return "", "", "", false
}

// prepareSuggestOutputPath validates --output shape/parent confinement before
// discovery. clean is the repository-relative form used later for the
// create-only write; on a shape rejection clean is "" so the envelope path
// stays repository-relative "when applicable" (issue #220/#210).
func prepareSuggestOutputPath(revisionSHA, root, outputPath string, outputSet bool) (clean string, fail SuggestionResult, ok bool) {
	if !outputSet {
		return "", SuggestionResult{}, true
	}
	clean, valErr := validateOutputPath(root, outputPath)
	if valErr != nil {
		return "", suggestFailureBeforeDiscovery(revisionSHA, SuggestDiagOutputInvalid, clean, valErr.Error()), false
	}
	return clean, SuggestionResult{}, true
}
