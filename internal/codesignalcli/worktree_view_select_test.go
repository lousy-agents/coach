package codesignalcli

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func dropTestFiles(files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) {
	var kept []gitrepo.SelectedFile
	for _, file := range files {
		if !strings.HasSuffix(file.Path, "_test.go") {
			kept = append(kept, file)
		}
	}
	return kept, nil
}

func selectionViewOverFiles(t *testing.T, porcelain string, files fstest.MapFS) WorktreeView {
	t.Helper()
	return newWorktreeViewForTest(t, worktreeViewDeps{
		status:    statusFromPorcelain(porcelainZ(strings.Split(porcelain, "\n")...), nil),
		committed: fstest.MapFS{"go.mod": {Data: []byte("module m\n")}},
		disk:      foldedNameDisk{files: files},
	})
}

func disclosedPaths(diagnostics []codesignal.Diagnostic, kind string) []string {
	var paths []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			paths = append(paths, diagnostic.Path)
		}
	}
	return paths
}

// The disclosure must not report a dirty file the scope filter dropped as one
// the scan failed to analyze. Selecting is the step that learns what the filter
// dropped, so a caller cannot forget to tell the view.
func TestSelectDiffRecordsTheFilesTheScopeFilterDropped(t *testing.T) {
	view := selectionViewOverFiles(t, "?? pkg/keep.go\n?? pkg/skip_test.go", fstest.MapFS{
		"pkg/keep.go":      {Data: []byte("package pkg\n")},
		"pkg/skip_test.go": {Data: []byte("package pkg\n")},
	})

	selection, err := view.SelectDiff("HEAD", nil, nil, nil, dropTestFiles)

	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Files) != 1 || selection.Files[0].Path != "pkg/keep.go" {
		t.Fatalf("Files = %+v, want only pkg/keep.go", selection.Files)
	}
	got := disclosedPaths(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeChangesNotAnalyzed)
	if len(got) != 1 || got[0] != "pkg/keep.go" {
		t.Errorf("unanalyzed paths = %v, want only pkg/keep.go: pkg/skip_test.go was excluded by the scope filter", got)
	}
}

func TestSelectBaselineRecordsTheFilesTheScopeFilterDropped(t *testing.T) {
	view := selectionViewOverFiles(t, "?? pkg/keep.go\n?? pkg/skip_test.go", fstest.MapFS{
		"pkg/keep.go":      {Data: []byte("package pkg\n")},
		"pkg/skip_test.go": {Data: []byte("package pkg\n")},
	})

	selection, err := view.SelectBaseline(nil, codesignal.Coverage{}, dropTestFiles)

	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Files) != 1 || selection.Files[0].Path != "pkg/keep.go" {
		t.Fatalf("Files = %+v, want only pkg/keep.go", selection.Files)
	}
	got := disclosedPaths(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeChangesNotAnalyzed)
	if len(got) != 1 || got[0] != "pkg/keep.go" {
		t.Errorf("unanalyzed paths = %v, want only pkg/keep.go", got)
	}
}
