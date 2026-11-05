package codesignalcli

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// unknownMergeBase is a revision no repository holds, so the committed diff of
// an added file against it fails.
const unknownMergeBase = "0000000000000000000000000000000000000000"

// analyzeAddedFileWhoseDiffFails analyzes a committed added file for which git
// cannot produce the diff that gives its changed ranges.
func analyzeAddedFileWhoseDiffFails(t *testing.T, view *WorktreeView) (*codesignal.FileChange, []codesignal.Diagnostic) {
	t.Helper()
	dir := gitfixture.Init(t)
	headSHA := gitfixture.CommitFile(t, dir, "a.go", "package a\n\nfunc A() {}\n")
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		t.Fatalf("NewAnalyzer: %v", err)
	}
	added := gitrepo.SelectedFile{Path: "a.go", Status: "added", Language: semantics.LanguageGo}

	change, diagnostics, stalled := analyzeAddedOrModifiedFile(gitrepo.RunBytes, context.Background(), analyzer, dir, headSHA, unknownMergeBase, added, view)
	if stalled != nil {
		t.Fatalf("analyzeAddedOrModifiedFile = %v; want a diff that fails to be a diagnostic and not an error", stalled)
	}
	return change, diagnostics
}

func TestCommittedScanLeavesTheChangedRangesOfAnAddedFileEmptyWhenItsDiffFails(t *testing.T) {
	change, diagnostics := analyzeAddedFileWhoseDiffFails(t, nil)

	if change == nil {
		t.Fatal("the added file was not analyzed")
	}
	if len(change.ChangedRanges) != 0 {
		t.Errorf("ChangedRanges = %v, want none: only a --worktree scan may treat every line of a file as changed", change.ChangedRanges)
	}
	if !hasDiagnostic(diagnostics, "a.go", "diff_analysis_failed") {
		t.Errorf("diagnostics = %v, want diff_analysis_failed for a.go", diagnostics)
	}
}

func TestWorktreeScanTreatsEveryLineOfAnAddedFileAsChangedWhenItsDiffFails(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{committed: fstest.MapFS{}, disk: absentDisk{}})

	change, diagnostics := analyzeAddedFileWhoseDiffFails(t, &view)

	if change == nil {
		t.Fatal("the added file was not analyzed")
	}
	if len(change.ChangedRanges) != 1 || change.ChangedRanges[0].StartRow != 0 || change.ChangedRanges[0].EndRow < 2 {
		t.Errorf("ChangedRanges = %v, want the whole file as one range", change.ChangedRanges)
	}
	if !hasDiagnostic(diagnostics, "a.go", "diff_analysis_failed") {
		t.Errorf("diagnostics = %v, want diff_analysis_failed for a.go", diagnostics)
	}
}
