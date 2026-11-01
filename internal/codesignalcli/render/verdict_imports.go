package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// countUnresolvedImportDiagnostics counts both sides of a diff, where a
// base-side diagnostic carries the "base_" kind prefix. An edge unresolved at
// the same file and line on both sides is one import, not two.
func countUnresolvedImportDiagnostics(diagnostics []codesignal.Diagnostic) int {
	type unresolvedEdge struct {
		path, message string
		row           uint
	}
	seen := make(map[unresolvedEdge]struct{})
	for _, d := range diagnostics {
		if strings.TrimPrefix(d.Kind, "base_") != codesignal.DiagKindProjectImportUnresolved {
			continue
		}
		edge := unresolvedEdge{path: d.Path, message: d.Message}
		if d.Location != nil {
			edge.row = d.Location.StartRow
		}
		seen[edge] = struct{}{}
	}
	return len(seen)
}

func unresolvedImportsClause(count int) string {
	if count == 1 {
		return "1 import did not resolve, so layer rules cannot tell whether it crosses a layer"
	}
	return fmt.Sprintf("%d imports did not resolve, so layer rules cannot tell whether they cross a layer", count)
}
