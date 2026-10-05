package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func renderDiagnostic(b *strings.Builder, diagnostic codesignal.Diagnostic) {
	fmt.Fprintf(b, "path: %s, kind: %s, message: %s\n", diagnostic.Path, diagnostic.Kind, diagnostic.Message)
}

func renderDiagnosticsSection(b *strings.Builder, diagnostics []codesignal.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	b.WriteString("\nDiagnostics:\n")
	for _, diagnostic := range diagnostics {
		renderDiagnostic(b, diagnostic)
	}
}

func hasProjectLifecycleDiagnostic(diagnostics []codesignal.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Kind == codesignal.DiagKindProjectCoverageIncomplete || d.Kind == codesignal.DiagKindProjectLifecycleIndeterminate {
			return true
		}
	}
	return false
}
