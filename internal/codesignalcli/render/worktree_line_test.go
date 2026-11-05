package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func worktreeReport(counts codesignal.WorktreeScope, diagnostics ...codesignal.Diagnostic) *codesignal.Report {
	counts.Revision = "0123456789abcdef"
	return &codesignal.Report{
		Scope:       codesignal.Scope{Worktree: &counts},
		Diagnostics: diagnostics,
	}
}

func worktreeLineOf(t *testing.T, report *codesignal.Report) string {
	t.Helper()
	var b strings.Builder
	renderWorktreeLine(&b, report)
	return b.String()
}

func TestWorktreeLineSaysNoUncommittedChangesOnlyWhenTheCountsWereRead(t *testing.T) {
	got := worktreeLineOf(t, worktreeReport(codesignal.WorktreeScope{}))

	if want := "worktree: no uncommitted changes on revision 0123456789ab.\n"; got != want {
		t.Errorf("a clean tree read without a failure rendered %q, want %q", got, want)
	}
}

func TestWorktreeLineDoesNotClaimNoUncommittedChangesWhenTheStatusReadFailed(t *testing.T) {
	failure := codesignal.Diagnostic{Kind: codesignal.DiagKindWorktreeStatusCheckFailed, Message: "working tree status check failed: timed out"}

	got := worktreeLineOf(t, worktreeReport(codesignal.WorktreeScope{}, failure))

	if strings.Contains(got, "no uncommitted changes") {
		t.Errorf("zero counts from a failed status read were reported as a clean tree: %q", got)
	}
	if !strings.Contains(got, "could not be read") {
		t.Errorf("the counts line does not say the counts are unknown: %q", got)
	}
}

func TestWorktreeLineKeepsCountsThatWereReadBesideAStatusFailure(t *testing.T) {
	failure := codesignal.Diagnostic{Kind: codesignal.DiagKindWorktreeStatusCheckFailed, Message: "working tree status check failed: sparse state"}

	got := worktreeLineOf(t, worktreeReport(codesignal.WorktreeScope{Modified: 2}, failure))

	if want := "worktree: 2 modified, 0 added, 0 deleted, 0 untracked on revision 0123456789ab.\n"; got != want {
		t.Errorf("counts read before an unrelated failure rendered %q, want %q", got, want)
	}
}
