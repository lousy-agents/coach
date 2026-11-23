package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// hiddenReason is one cause of a worktree_scan_incomplete entry. cause is what
// the verdict says when in-scope source may be hidden; disclosure is what the
// report says when none is, which leaves the verdict unqualified.
type hiddenReason struct {
	name       string
	cause      string
	disclosure string
}

const (
	sparseCheckoutRemedy  = "run `git sparse-checkout disable` to read them"
	skipWorktreeRemedy    = "run `git update-index --no-skip-worktree <path>` to read one"
	assumeUnchangedRemedy = "run `git update-index --no-assume-unchanged <path>` to read one"
)

var (
	sparseCheckoutReason = hiddenReason{
		name:       "sparse_checkout",
		cause:      "sparse checkout is enabled, so the worktree scan uses the committed version of the files outside the checkout, not their working-tree bytes (" + sparseCheckoutRemedy + ")",
		disclosure: hiddenFromStatus("sparse checkout", sparseCheckoutRemedy),
	}
	skipWorktreeReason = hiddenReason{
		name:       "skip_worktree",
		cause:      "skip-worktree entries are present, so the worktree scan uses the committed version of those files, not their working-tree bytes (" + skipWorktreeRemedy + ")",
		disclosure: hiddenFromStatus("skip-worktree", skipWorktreeRemedy),
	}
	assumeUnchangedReason = hiddenReason{
		name:       "assume_unchanged",
		cause:      "assume-unchanged entries are present, so the worktree scan uses the committed version of those files, not their working-tree bytes (" + assumeUnchangedRemedy + ")",
		disclosure: hiddenFromStatus("assume-unchanged", assumeUnchangedRemedy),
	}
)

func hiddenFromStatus(label, remedy string) string {
	return label + " keeps tracked paths out of git status; no in-scope source is hidden, so the verdict is not qualified (" + remedy + ")"
}

func (r hiddenReason) message(hidesInScope string) string {
	if hidesInScope != codesignal.HidesInScopeNo {
		return r.cause
	}
	return r.disclosure
}

// entry leaves Path empty: a path would count as an unanalyzed file, and these
// entries name directories.
func (r hiddenReason) entry(root, hidesInScope string) codesignal.Diagnostic {
	return codesignal.Diagnostic{
		Kind:         codesignal.DiagKindWorktreeScanIncomplete,
		Reason:       r.name,
		Root:         root,
		HidesInScope: hidesInScope,
		Message:      r.message(hidesInScope),
	}
}

// hiddenPathDiagnostics reports each reason git hides tracked paths from the
// status, with one entry per root directory. A sparse checkout owns the
// skip-worktree entries it set, so they are not reported a second time. A sparse
// checkout that hides nothing is still disclosed, with no root.
func (v *WorktreeView) hiddenPathDiagnostics(state gitrepo.SparseState, policyPath string) []codesignal.Diagnostic {
	skipWorktree := state.SkipWorktree
	var diagnostics []codesignal.Diagnostic
	if state.SparseCheckout {
		diagnostics = v.hiddenReasonDiagnostics(sparseCheckoutReason, skipWorktree, state.Tracked, policyPath)
		skipWorktree = nil
	}
	if len(skipWorktree) > 0 {
		diagnostics = append(diagnostics, v.hiddenReasonDiagnostics(skipWorktreeReason, skipWorktree, state.Tracked, policyPath)...)
	}
	if len(state.AssumeUnchanged) > 0 {
		diagnostics = append(diagnostics, v.hiddenReasonDiagnostics(assumeUnchangedReason, state.AssumeUnchanged, state.Tracked, policyPath)...)
	}
	return diagnostics
}

func (v *WorktreeView) hiddenReasonDiagnostics(reason hiddenReason, hidden, tracked []string, policyPath string) []codesignal.Diagnostic {
	byRoot := hiddenPathsByRoot(hidden, tracked)
	if len(byRoot) == 0 {
		byRoot = map[string][]string{"": nil}
	}
	return reasonDiagnostics(reason, v.hidesInScope(byRoot, policyPath))
}
