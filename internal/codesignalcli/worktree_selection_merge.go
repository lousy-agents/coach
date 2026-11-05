package codesignalcli

import (
	"slices"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// The functions below merge what the committed selection already reported with
// what the working tree adds, so a rename or a diagnostic is not reported twice
// or kept after the working tree removed what it described.

func withoutVanished(diagnostics []codesignal.Diagnostic, vanished map[string]bool) []codesignal.Diagnostic {
	return slices.DeleteFunc(slices.Clone(diagnostics), func(diagnostic codesignal.Diagnostic) bool {
		return diagnostic.Kind == "continuity_not_determined" && vanished[diagnostic.Path]
	})
}

func withoutVanishedPairs(continuity []codesignal.PathContinuity, vanished map[string]bool) []codesignal.PathContinuity {
	return slices.DeleteFunc(slices.Clone(continuity), func(pair codesignal.PathContinuity) bool { return vanished[pair.Path] })
}

// appendUnreported adds the extra diagnostics that no diagnostic of the same
// kind already reports for the same path, so a file the committed diff and the
// working tree both changed is named once.
func appendUnreported(diagnostics, extra []codesignal.Diagnostic) []codesignal.Diagnostic {
	type key struct{ kind, path string }
	reported := make(map[key]bool, len(diagnostics))
	for _, diagnostic := range diagnostics {
		reported[key{diagnostic.Kind, diagnostic.Path}] = true
	}
	for _, diagnostic := range extra {
		if diagnostic.Path == "" || !reported[key{diagnostic.Kind, diagnostic.Path}] {
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	return diagnostics
}
