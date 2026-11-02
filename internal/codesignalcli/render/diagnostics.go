package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func renderDiagnostic(b *strings.Builder, diagnostic codesignal.Diagnostic) {
	fmt.Fprintf(b, "path: %s, kind: %s, message: %s", diagnostic.Path, diagnostic.Kind, diagnostic.Message)
	if diagnostic.Side != "" {
		fmt.Fprintf(b, ", side: %s", diagnostic.Side)
	}
	if diagnostic.Revision != "" {
		fmt.Fprintf(b, ", revision: %s", diagnostic.Revision)
	}
	b.WriteByte('\n')
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
