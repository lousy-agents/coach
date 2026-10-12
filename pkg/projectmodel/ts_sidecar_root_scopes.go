package projectmodel

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

func rootScopesFromWire(in []projectbridge.RootScopeFact) []RootScope {
	if len(in) == 0 {
		return nil
	}
	scopes := make([]RootScope, 0, len(in))
	for _, rs := range in {
		scopes = append(scopes, RootScope{
			Root:            rs.Root,
			CandidateFiles:  rs.CandidateFiles,
			AnalyzedFiles:   rs.AnalyzedFiles,
			AnalyzedPaths:   rs.AnalyzedPaths,
			UnanalyzedPaths: rs.UnanalyzedPaths,
		})
	}
	return scopes
}

// rootScopeIncompleteDiagnostics never reads reachability diagnostics --
// model completeness and reachability completeness are independent axes.
// The per-root fallback below covers a RootScope with counts but no path
// lists (a sidecar response predating UnanalyzedPaths).
func rootScopeIncompleteDiagnostics(scopes []RootScope) []Diagnostic {
	var diags []Diagnostic
	for _, scope := range scopes {
		if scope.AnalyzedFiles >= scope.CandidateFiles {
			continue
		}
		if len(scope.UnanalyzedPaths) > 0 {
			diags = append(diags, incompletePathDiagnostics(scope)...)
			continue
		}
		diags = append(diags, Diagnostic{
			Code:    DiagRootScopeIncomplete,
			Message: fmt.Sprintf("root %q: only %d of %d candidate files were incorporated into the import model", scope.Root, scope.AnalyzedFiles, scope.CandidateFiles),
			Path:    scope.Root,
		})
	}
	return diags
}

func incompletePathDiagnostics(scope RootScope) []Diagnostic {
	diags := make([]Diagnostic, 0, len(scope.UnanalyzedPaths))
	for _, path := range scope.UnanalyzedPaths {
		diags = append(diags, Diagnostic{
			Code:    DiagRootScopeIncomplete,
			Message: fmt.Sprintf("root %q: candidate file %q was never incorporated into the import model", scope.Root, path),
			Path:    path,
		})
	}
	return diags
}
