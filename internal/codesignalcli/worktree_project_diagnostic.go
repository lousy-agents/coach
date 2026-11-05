package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func appendProjectWorktreeDiagnostic(diagnostics []codesignal.Diagnostic, status WorktreeStatus, roots []string, policy policyref.Ref) []codesignal.Diagnostic {
	entries, err := status.Entries, status.Err
	if err != nil {
		if hasDiagnosticKind(diagnostics, codesignal.DiagKindWorktreeStatusCheckFailed) ||
			hasDiagnosticKind(diagnostics, codesignal.DiagKindWorktreeNotClean) {
			return diagnostics
		}
		return appendDiagnosticCopy(diagnostics, worktreeStatusFailureDiagnostic(err))
	}
	if !hasRelevantDirtyWorktree(entries, roots, policy.Path) {
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

func hasRelevantDirtyWorktree(entries []gitrepo.WorktreeEntry, roots []string, policyPath string) bool {
	for _, entry := range entries {
		if projectcheck.IsRelevantDirtyPath(entry.Path, roots, policyPath) {
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
