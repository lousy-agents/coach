package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func renderDiagnostic(b *strings.Builder, diagnostic codesignal.Diagnostic, repeatsMessage bool) {
	fmt.Fprintf(b, "path: %s, kind: %s", diagnostic.Path, diagnostic.Kind)
	if !repeatsMessage {
		fmt.Fprintf(b, ", message: %s", diagnostic.Message)
	}
	if diagnostic.Side != "" {
		fmt.Fprintf(b, ", side: %s", diagnostic.Side)
	}
	if diagnostic.Revision != "" {
		fmt.Fprintf(b, ", revision: %s", diagnostic.Revision)
	}
	if diagnostic.Reason != "" {
		fmt.Fprintf(b, ", reason: %s", diagnostic.Reason)
	}
	if diagnostic.Root != "" {
		fmt.Fprintf(b, ", root: %s", terminalText(diagnostic.Root+"/"))
	}
	if diagnostic.RootsNotListed > 0 {
		fmt.Fprintf(b, ", %d more %s not listed", diagnostic.RootsNotListed, rootNoun(diagnostic.RootsNotListed))
	}
	if diagnostic.HidesInScope != "" {
		fmt.Fprintf(b, ", hides_in_scope: %s", diagnostic.HidesInScope)
	}
	b.WriteByte('\n')
}

func rootNoun(n int) string {
	if n == 1 {
		return "root"
	}
	return "roots"
}

// renderDiagnosticsSection states a reason's message once: a reason with many
// roots has an entry per root, and repeating its sentence on each would bury
// the roots in it.
func renderDiagnosticsSection(b *strings.Builder, diagnostics []codesignal.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	b.WriteString("\nDiagnostics:\n")
	stated := map[[2]string]bool{}
	for _, diagnostic := range diagnostics {
		repeats := false
		if diagnostic.Reason != "" {
			key := [2]string{diagnostic.Reason, diagnostic.Message}
			repeats, stated[key] = stated[key], true
		}
		renderDiagnostic(b, diagnostic, repeats)
	}
}
