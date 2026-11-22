package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// hiddenReason is one way git keeps tracked paths out of `git status`. cause is
// what the verdict says when in-scope source may be among them; the disclosure
// that does not qualify the verdict is built from label and remedy.
type hiddenReason struct {
	name   string
	label  string
	cause  string
	remedy string
}

const (
	sparseCheckoutRemedy  = "run `git sparse-checkout disable` to read them"
	skipWorktreeRemedy    = "run `git update-index --no-skip-worktree <path>` to read one"
	assumeUnchangedRemedy = "run `git update-index --no-assume-unchanged <path>` to read one"
)

var (
	sparseCheckoutReason = hiddenReason{
		name: "sparse_checkout", label: "sparse checkout", remedy: sparseCheckoutRemedy,
		cause: "sparse checkout is enabled, so the worktree scan uses the committed version of the files outside the checkout, not their working-tree bytes (" + sparseCheckoutRemedy + ")",
	}
	skipWorktreeReason = hiddenReason{
		name: "skip_worktree", label: "skip-worktree", remedy: skipWorktreeRemedy,
		cause: "skip-worktree entries are present, so the worktree scan uses the committed version of those files, not their working-tree bytes (" + skipWorktreeRemedy + ")",
	}
	assumeUnchangedReason = hiddenReason{
		name: "assume_unchanged", label: "assume-unchanged", remedy: assumeUnchangedRemedy,
		cause: "assume-unchanged entries are present, so the worktree scan uses the committed version of those files, not their working-tree bytes (" + assumeUnchangedRemedy + ")",
	}
)

func (r hiddenReason) message(hidesInScope string) string {
	if hidesInScope != codesignal.HidesInScopeNo {
		return r.cause
	}
	return r.label + " keeps tracked paths out of git status; no in-scope source is hidden, so the verdict is not qualified (" + r.remedy + ")"
}

// hiddenPathDiagnostics reports each reason git hides tracked paths from the
// status, with one entry per root directory. A sparse checkout owns the
// skip-worktree entries it set, so they are not reported a second time. A sparse
// checkout that hides nothing is still disclosed, with no root.
func (v *WorktreeView) hiddenPathDiagnostics(state gitrepo.SparseState) []codesignal.Diagnostic {
	skipWorktree := state.SkipWorktree
	var diagnostics []codesignal.Diagnostic
	if state.SparseCheckout {
		diagnostics = v.hiddenReasonDiagnostics(sparseCheckoutReason, skipWorktree, state.Tracked)
		skipWorktree = nil
	}
	if len(skipWorktree) > 0 {
		diagnostics = append(diagnostics, v.hiddenReasonDiagnostics(skipWorktreeReason, skipWorktree, state.Tracked)...)
	}
	if len(state.AssumeUnchanged) > 0 {
		diagnostics = append(diagnostics, v.hiddenReasonDiagnostics(assumeUnchangedReason, state.AssumeUnchanged, state.Tracked)...)
	}
	return diagnostics
}

// hiddenReasonDiagnostics leaves Path empty: a path would count as an
// unanalyzed file, and these entries name directories.
func (v *WorktreeView) hiddenReasonDiagnostics(reason hiddenReason, hidden, tracked []string) []codesignal.Diagnostic {
	entry := codesignal.Diagnostic{
		Kind:         codesignal.DiagKindWorktreeScanIncomplete,
		Reason:       reason.name,
		HidesInScope: v.hidesInScope(hidden),
	}
	entry.Message = reason.message(entry.HidesInScope)
	roots := hiddenRoots(hidden, tracked)
	if len(roots) == 0 {
		return []codesignal.Diagnostic{entry}
	}
	diagnostics := make([]codesignal.Diagnostic, len(roots))
	for i, root := range roots {
		diagnostics[i] = entry
		diagnostics[i].Root = root
	}
	return diagnostics
}
