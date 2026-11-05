package codesignalcli

import (
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A changed file in an unsupported language is named by an unsupported_language
// diagnostic, from the working-tree selection or from the committed diff, and the
// disclosure must not list it again as a dirty file the scan ignored. A dirty file
// nothing names is still listed.
func TestSelectDiffRecordsThePathsAnUnsupportedLanguageDiagnosticNames(t *testing.T) {
	tests := []struct {
		name       string
		porcelain  string
		committed  []codesignal.Diagnostic
		wantListed bool
	}{
		{"named by the working-tree selection", "?? notes.txt", nil, false},
		{"named by the committed diff, which the selection leaves out under node_modules", " M node_modules/pkg/readme.txt", []codesignal.Diagnostic{{Kind: "unsupported_language", Path: "node_modules/pkg/readme.txt"}}, false},
		{"named by nothing, under node_modules", "?? node_modules/pkg/readme.txt", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertListedAsUnread(t, tt.porcelain, tt.committed, tt.wantListed)
		})
	}
}

func assertListedAsUnread(t *testing.T, porcelain string, committed []codesignal.Diagnostic, wantListed bool) {
	t.Helper()
	view := selectionViewOverFiles(t, porcelain, fstest.MapFS{})
	if _, err := view.SelectDiff("HEAD", nil, committed, nil, dropTestFiles); err != nil {
		t.Fatal(err)
	}

	listed := countKind(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeNotClean) > 0

	if listed != wantListed {
		t.Errorf("disclosure listed the file as unread = %v, want %v", listed, wantListed)
	}
}

func countKind(diagnostics []codesignal.Diagnostic, kind string) int {
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			count++
		}
	}
	return count
}
