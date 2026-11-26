package codesignal

import "github.com/lousy-agents/coach/pkg/domain"

// requiredLayerUnmatchedDiagnostics carries the cause behind an ambiguous
// required layer to the report, next to the incomplete-coverage diagnostic.
func requiredLayerUnmatchedDiagnostics(coverage domain.Coverage) []Diagnostic {
	var diagnostics []Diagnostic
	for _, diagnostic := range coverage.Diagnostics {
		if diagnostic.Code == domain.DiagLayerBypassRequiredLayerUnmatched {
			diagnostics = append(diagnostics, Diagnostic{Kind: diagnostic.Code, Message: diagnostic.Message})
		}
	}
	return diagnostics
}
