package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func appendProjectWorktreeDiagnostic(diagnostics []codesignal.Diagnostic, status WorktreeStatus, roots []string, policy policyref.Ref, absentPolicyPaths ...string) []codesignal.Diagnostic {
	entries, err := status.Entries, status.Err
	if err != nil {
		_, failureReported := codesignal.DiagnosticOfKind(diagnostics, codesignal.DiagKindWorktreeStatusCheckFailed)
		_, notCleanReported := codesignal.DiagnosticOfKind(diagnostics, codesignal.DiagKindWorktreeNotClean)
		if failureReported || notCleanReported {
			return diagnostics
		}
		return appendDiagnosticCopy(diagnostics, worktreeStatusFailureDiagnostic(err))
	}
	if !hasRelevantDirtyWorktree(entries, roots, policy.Path, absentPolicyPaths) {
		return diagnostics
	}
	if _, reported := codesignal.DiagnosticOfKind(diagnostics, codesignal.DiagKindWorktreeReportReflectsCommittedHEAD); reported {
		return diagnostics
	}
	return appendDiagnosticCopy(diagnostics, codesignal.Diagnostic{
		Kind:    codesignal.DiagKindWorktreeReportReflectsCommittedHEAD,
		Message: "report reflects committed HEAD",
	})
}

func hasRelevantDirtyWorktree(entries []gitrepo.WorktreeEntry, roots []string, policyPath string, absentPolicyPaths []string) bool {
	for _, entry := range entries {
		if gitrepo.SamePath(entry.Path, policyPath) || isAbsentPolicy(entry.Path, absentPolicyPaths) {
			continue
		}
		if projectcheck.IsRelevantDirtyPath(entry.Path, roots, policyPath) {
			return true
		}
	}
	return false
}

func appendDiagnosticCopy(diagnostics []codesignal.Diagnostic, extra codesignal.Diagnostic) []codesignal.Diagnostic {
	return append(append([]codesignal.Diagnostic(nil), diagnostics...), extra)
}
