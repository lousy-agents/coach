package main

import "github.com/lousy-agents/coach/pkg/codesignal"

// withoutPathsAlreadyUnusable drops the on-disk refusals of a path that discovery already reported as unusable, so
// that one cause is named once.
func withoutPathsAlreadyUnusable(refusals, reported []codesignal.Diagnostic) []codesignal.Diagnostic {
	var kept []codesignal.Diagnostic
	for _, refusal := range refusals {
		if !reportedUnusable(reported, refusal.Path) {
			kept = append(kept, refusal)
		}
	}
	return kept
}

func reportedUnusable(reported []codesignal.Diagnostic, path string) bool {
	for _, diag := range reported {
		if diag.Kind == "project_config_unusable" && diag.Path == path {
			return true
		}
	}
	return false
}
