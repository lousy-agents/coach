package codesignalcli

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A git that cannot be told not to fetch is given the status that does not look into submodules, so an edit inside a
// submodule is not in it. A scan that said nothing would report a tree with such an edit as clean.
func TestWorktreeViewDisclosesEverySubmoduleUnderGitVersionWhenTheStatusSkippedThem(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status: WorktreeStatus{
			SubmodulesSkippedBy: "2.43.0",
			Entries:             []gitrepo.WorktreeEntry{{Code: " M", Path: "vendored"}},
		},
		committed: fstest.MapFS{},
		disk:      absentDisk{},
		gitlinks:  []string{"vendored", "clean.go"},
	})
	view.scope = keepNothing

	entries := scanIncompleteEntries(view.Disclosure(".", "HEAD", ""))

	if len(entries) != 2 {
		t.Fatalf("worktree_scan_incomplete diagnostics = %+v, want one per submodule and no submodule entry", entries)
	}
	for i, root := range []string{"clean.go", "vendored"} {
		got := entries[i]
		if got.Reason != "git_version" || got.Root != root || got.HidesInScope != codesignal.HidesInScopeNo || got.Path != "" {
			t.Errorf("entry %d = %+v, want git_version at %q hiding no in-scope source and no path", i, got, root)
		}
		if !strings.Contains(got.Message, "git 2.43.0") {
			t.Errorf("message %q does not name the git version", got.Message)
		}
	}
}

func TestWorktreeViewQualifiesAGitlinkNamedLikeSourceTheScopeKeeps(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    WorktreeStatus{SubmodulesSkippedBy: "2.43.0"},
		committed: fstest.MapFS{},
		disk:      absentDisk{},
		gitlinks:  []string{"x.go", "lib"},
	})
	view.scope = func(files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) { return files, nil }

	got := map[string]string{}
	for _, entry := range scanIncompleteEntries(view.Disclosure(".", "HEAD", "")) {
		got[entry.Root] = entry.HidesInScope
	}

	if want := map[string]string{"x.go": codesignal.HidesInScopeYes, "lib": codesignal.HidesInScopeNo}; !reflect.DeepEqual(got, want) {
		t.Errorf("hides_in_scope by root = %v, want %v", got, want)
	}
}

func TestWorktreeViewCannotTellWhetherAGitlinkNamedLikeSourceIsInScopeWithoutAFilter(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    WorktreeStatus{SubmodulesSkippedBy: "2.43.0"},
		committed: fstest.MapFS{},
		disk:      absentDisk{},
		gitlinks:  []string{"x.go"},
	})

	entries := scanIncompleteEntries(view.Disclosure(".", "HEAD", ""))

	if len(entries) != 1 || entries[0].HidesInScope != codesignal.HidesInScopeUnknown {
		t.Errorf("entries = %+v, want one that is unknown", entries)
	}
}

func scanIncompleteEntries(diagnostics []codesignal.Diagnostic) []codesignal.Diagnostic {
	return slices.DeleteFunc(slices.Clone(diagnostics), func(d codesignal.Diagnostic) bool {
		return d.Kind != codesignal.DiagKindWorktreeScanIncomplete
	})
}

func TestWorktreeViewNamesNoSkippedSubmodulesWhenThereIsNoneToSkipOrNoneWasSkipped(t *testing.T) {
	assertNoScanIncomplete(t, WorktreeStatus{SubmodulesSkippedBy: "2.43.0"}, nil)
	assertNoScanIncomplete(t, WorktreeStatus{}, []string{"vendored"})
}

func assertNoScanIncomplete(t *testing.T, status WorktreeStatus, gitlinks []string) {
	t.Helper()
	view := newWorktreeViewForTest(t, worktreeViewDeps{status: status, committed: fstest.MapFS{}, disk: absentDisk{}, gitlinks: gitlinks})

	if got := countKind(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeScanIncomplete); got != 0 {
		t.Errorf("status %+v with gitlinks %v: worktree_scan_incomplete diagnostics = %d, want none", status, gitlinks, got)
	}
}
