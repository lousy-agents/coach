package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// listWorktreeStatus is the worktree-status read behind the scan's
// worktree diagnostics. Tests may replace it with canned porcelain entries.
var listWorktreeStatus = gitrepo.WorktreeStatus

func appendProjectWorktreeDiagnostic(diagnostics []codesignal.Diagnostic, dir string, roots []string, configPath string) []codesignal.Diagnostic {
	entries, err := listWorktreeStatus(dir)
	if err != nil {
		if hasDiagnosticKind(diagnostics, codesignal.DiagKindWorktreeStatusCheckFailed) ||
			hasDiagnosticKind(diagnostics, codesignal.DiagKindWorktreeNotClean) {
			return diagnostics
		}
		return appendDiagnosticCopy(diagnostics, worktreeStatusFailureDiagnostic(err))
	}
	if !hasRelevantDirtyWorktree(entries, roots, configPath) {
		return diagnostics
	}
	if hasDiagnosticKind(diagnostics, codesignal.DiagKindWorktreeReportReflectsCommittedHEAD) {
		return diagnostics
	}
	return appendDiagnosticCopy(diagnostics, codesignal.Diagnostic{
		Kind:    codesignal.DiagKindWorktreeReportReflectsCommittedHEAD,
		Message: "report reflects committed HEAD",
	})
}

func hasRelevantDirtyWorktree(entries []gitrepo.WorktreeEntry, roots []string, configPath string) bool {
	for _, entry := range entries {
		if isRelevantDirtyPath(entry.Path, roots, configPath) {
			return true
		}
	}
	return false
}

func hasDiagnosticKind(diagnostics []codesignal.Diagnostic, kind string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			return true
		}
	}
	return false
}

func appendDiagnosticCopy(diagnostics []codesignal.Diagnostic, extra codesignal.Diagnostic) []codesignal.Diagnostic {
	return append(append([]codesignal.Diagnostic(nil), diagnostics...), extra)
}
