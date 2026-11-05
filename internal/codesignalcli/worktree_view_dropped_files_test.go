package codesignalcli

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestCoverageAfterDropsMarksHeadCoverageIncompleteWhenADroppedFileLiesInsideTheRoots(t *testing.T) {
	tests := []struct {
		name         string
		outcome      worktreeoverlay.Outcome
		roots        []string
		wantComplete bool
	}{
		{"a symbolic link under a root", worktreeoverlay.Outcome{SymlinkSkipped: []string{"pkg/a/x.go"}}, []string{"pkg/a"}, false},
		{"a file that could not be read under a root", worktreeoverlay.Outcome{Unreadable: []worktreeoverlay.Unreadable{{Path: "pkg/a/x.go"}}}, []string{"pkg/a"}, false},
		{"a file that changed while it was read under a root", worktreeoverlay.Outcome{Changed: []string{"pkg/a/x.go"}}, []string{"pkg/a"}, false},
		{"the root itself replaced by a symbolic link", worktreeoverlay.Outcome{SymlinkSkipped: []string{"pkg/a"}}, []string{"pkg/a"}, false},
		{"a root written with a trailing slash", worktreeoverlay.Outcome{Changed: []string{"pkg/a/x.go"}}, []string{"pkg/a/"}, false},
		{"a dropped file under the repository root", worktreeoverlay.Outcome{Changed: []string{"x.go"}}, []string{"."}, false},
		{"no roots configured means every path is inside", worktreeoverlay.Outcome{Changed: []string{"x.go"}}, nil, false},
		{"a dropped file outside every root", worktreeoverlay.Outcome{SymlinkSkipped: []string{"tools/x.go"}}, []string{"pkg/a"}, true},
		{"a sibling whose name only starts like the root", worktreeoverlay.Outcome{Changed: []string{"pkg/ab/x.go"}}, []string{"pkg/a"}, true},
		{"nothing dropped", worktreeoverlay.Outcome{}, []string{"pkg/a"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompleteAfterDrops(t, tt.outcome, tt.roots, tt.wantComplete)
		})
	}
}

func assertCompleteAfterDrops(t *testing.T, outcome worktreeoverlay.Outcome, roots []string, wantComplete bool) {
	t.Helper()
	view := &WorktreeView{outcome: outcome}
	coverage := &projectmodel.Coverage{Complete: true}

	got := view.coverageAfterDrops(coverage, roots)

	if got.Complete != wantComplete {
		t.Errorf("Complete = %v, want %v for %+v under roots %v", got.Complete, wantComplete, outcome, roots)
	}
	if !coverage.Complete {
		t.Error("the backend's coverage was written to; the result must be a copy")
	}
}

// A file that grows between the stat and the read is dropped, so project
// analysis ran without it and its coverage cannot claim to be complete.
func TestCoverageAfterDropsSeesAFileThatChangedWhileItWasRead(t *testing.T) {
	disk := &scriptedDisk{beforeRead: scriptedInfo{size: 3}, afterRead: scriptedInfo{size: 4}, data: []byte("abc")}
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    statusFromPorcelain(porcelainZ(" M a.go"), nil),
		committed: fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		disk:      disk,
	})
	if _, err := view.OpenHead(".", "HEAD"); err != nil {
		t.Fatal(err)
	}

	got := view.coverageAfterDrops(&projectmodel.Coverage{Complete: true}, []string{"."})

	if got.Complete {
		t.Error("coverage stayed complete although a.go changed while it was read and was dropped")
	}
}

// A dirty go.mod that is a symbolic link on disk is dropped under its own
// diagnostic. It is neither read as configuration nor listed again as a change
// the scan left unread.
func TestADirtyConfigurationFileTheOverlayDroppedIsNamedOnce(t *testing.T) {
	link := &scriptedDisk{beforeRead: scriptedInfo{mode: fs.ModeSymlink}}
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    statusFromPorcelain(porcelainZ(" M go.mod"), nil),
		committed: fstest.MapFS{"go.mod": {Data: []byte("module m\n")}},
		disk:      link,
	})

	diagnostics := append(view.Diagnostics(".", "HEAD"), view.Disclosure(".", "HEAD", "")...)

	if got := countKind(diagnostics, codesignal.DiagKindWorktreeSymlinkSkipped); got != 1 {
		t.Errorf("symlink diagnostics = %d, want 1: %+v", got, diagnostics)
	}
	if got := countKind(diagnostics, codesignal.DiagKindWorktreeNotClean); got != 0 {
		t.Errorf("go.mod was listed again as an unread change: %+v", diagnostics)
	}
	if len(diagnostics) != 1 {
		t.Errorf("diagnostics = %+v, want only the symlink diagnostic", diagnostics)
	}
}
