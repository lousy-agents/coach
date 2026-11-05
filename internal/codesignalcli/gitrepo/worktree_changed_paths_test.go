package gitrepo_test

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestChangedPaths(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{"modified, added, deleted and untracked", " M a.go\x00A  b.go\x00 D c.go\x00?? d.go\x00", []string{"a.go", "b.go", "c.go", "d.go"}},
		{"a staged rename yields the new and the old path", "R  new.go\x00old.go\x00 M a.go\x00", []string{"new.go", "old.go", "a.go"}},
		{"a copy yields only the new path, because the source stays", "C  copy.go\x00src.go\x00", []string{"copy.go"}},
		{"a rename that is also edited yields both paths", "RM new.go\x00old.go\x00", []string{"new.go", "old.go"}},
		{"too-short records are skipped", "x\x00 M a.go\x00", []string{"a.go"}},
		{"empty output", "", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertChangedPaths(t, tt.output, tt.want)
		})
	}
}

func assertChangedPaths(t *testing.T, output string, want []string) {
	t.Helper()
	got := gitrepo.ChangedPaths(gitrepo.ParseWorktreeStatus([]byte(output)))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ChangedPaths(%q) paths = %v, want %v", output, got, want)
	}
}
