package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_worktreeDisclosurePart2Test_154(t *testing.T, tt struct {
	name   string
	output []byte
	err    error
	want   []codesignal.Diagnostic
}) {
	withDirtyWorktreeGit(t, func(string, ...string) ([]byte, error) {
		return tt.output, tt.err
	})

	got := WorkingTreeDisclosureDiagnostics("repo")
	if len(got) != len(tt.want) {
		t.Fatalf("diagnostics: got %d (%+v), want %d (%+v)", len(got), got, len(tt.want), tt.want)
	}
	for i := range tt.want {
		if got[i] != tt.want[i] {
			t.Errorf("diagnostic %d: got %+v, want %+v", i, got[i], tt.want[i])
		}
		if got[i].Path != "" {
			t.Errorf("diagnostic %d path: got %q, want empty so files_unanalyzed stays zero", i, got[i].Path)
		}
	}
}
