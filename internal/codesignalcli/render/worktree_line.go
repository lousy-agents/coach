package render

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// noChangesPhrase says no changes were found, but only as far as git status
// can see: when the scan recorded that git hides some of the working tree, an
// unqualified "no uncommitted changes" would contradict the cause beside it.
func noChangesPhrase(report *codesignal.Report) string {
	for _, d := range report.Diagnostics {
		if d.Kind == codesignal.DiagKindWorktreeScanIncomplete {
			return "git status reports no uncommitted changes"
		}
	}
	return "no uncommitted changes"
}

// worktreeCountsClause says what git status found. All-zero counts after a
// failed status read are not a clean tree: the zeros are the absence of a read,
// so the clause says the counts are unknown instead of "no uncommitted changes".
func worktreeCountsClause(report *codesignal.Report) string {
	worktree := report.Scope.Worktree
	if worktree.Modified+worktree.Added+worktree.Deleted+worktree.Untracked > 0 {
		return fmt.Sprintf("%d modified, %d added, %d deleted, %d untracked",
			worktree.Modified, worktree.Added, worktree.Deleted, worktree.Untracked)
	}
	if hasStatusCheckFailure(report.Diagnostics) {
		return "the uncommitted-change counts could not be read"
	}
	return noChangesPhrase(report)
}

func hasStatusCheckFailure(diagnostics []codesignal.Diagnostic) bool {
	return slices.ContainsFunc(diagnostics, func(d codesignal.Diagnostic) bool {
		return d.Kind == codesignal.DiagKindWorktreeStatusCheckFailed
	})
}

// renderWorktreeLine says, under the scope summary, what a --worktree scan read:
// the working-tree counts, the revision they sit on, the merge base of a diff
// scan, and the configuration files read from disk.
func renderWorktreeLine(b *strings.Builder, report *codesignal.Report) {
	worktree := report.Scope.Worktree
	if worktree == nil {
		return
	}
	fmt.Fprintf(b, "worktree: %s on revision %s", worktreeCountsClause(report), shortRevision(worktree.Revision))
	if report.Scope.Base != "" {
		fmt.Fprintf(b, ", compared with merge-base %s", shortRevision(report.Scope.Base))
	}
	if len(worktree.ConfigFiles) > 0 {
		fmt.Fprintf(b, "; configuration read from disk: %s", strings.Join(worktree.ConfigFiles, ", "))
	}
	b.WriteString(".\n")
}

func shortRevision(revision string) string {
	if len(revision) > 12 {
		return revision[:12]
	}
	return revision
}
