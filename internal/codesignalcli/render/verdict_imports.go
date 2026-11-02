package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// countDistinctEdgeDiagnostics counts the edges of one kind across both sides
// of a diff, where a base-side diagnostic carries the "base_" kind prefix. An
// edge reported at the same file, line and message on both sides is one edge,
// so each key contributes the larger of its two per-side counts; two identical
// calls on one line on a single side are still two.
func countDistinctEdgeDiagnostics(diagnostics []codesignal.Diagnostic, kind string) int {
	type edge struct {
		path, message string
		row           uint
	}
	type sideCounts struct{ head, base int }
	counts := make(map[edge]*sideCounts)
	for _, d := range diagnostics {
		trimmed := strings.TrimPrefix(d.Kind, "base_")
		if trimmed != kind {
			continue
		}
		e := edge{path: d.Path, message: d.Message}
		if d.Location != nil {
			e.row = d.Location.StartRow
		}
		c := counts[e]
		if c == nil {
			c = &sideCounts{}
			counts[e] = c
		}
		if trimmed != d.Kind {
			c.base++
		} else {
			c.head++
		}
	}
	total := 0
	for _, c := range counts {
		total += max(c.head, c.base)
	}
	return total
}

// unplacedImportClauses names every kind of import layer rules could not
// place, so each verdict branch states the same causes.
func unplacedImportClauses(diagnostics []codesignal.Diagnostic) []string {
	var clauses []string
	if n := countDistinctEdgeDiagnostics(diagnostics, codesignal.DiagKindProjectImportUnresolved); n > 0 {
		clauses = append(clauses, unresolvedImportsClause(n))
	}
	if n := countDistinctEdgeDiagnostics(diagnostics, codesignal.DiagKindProjectImportNonLiteral); n > 0 {
		clauses = append(clauses, nonLiteralLoadsClause(n))
	}
	return clauses
}

func unresolvedImportsClause(count int) string {
	if count == 1 {
		return "1 import did not resolve, so layer rules cannot tell whether it crosses a layer"
	}
	return fmt.Sprintf("%d imports did not resolve, so layer rules cannot tell whether they cross a layer", count)
}

func nonLiteralLoadsClause(count int) string {
	if count == 1 {
		return "1 dynamic load with a non-literal specifier was not evaluated, so layer rules cannot tell whether it crosses a layer"
	}
	return fmt.Sprintf("%d dynamic loads with a non-literal specifier were not evaluated, so layer rules cannot tell whether they cross a layer", count)
}
