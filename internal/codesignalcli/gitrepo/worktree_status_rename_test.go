package gitrepo_test

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestParseWorktreeStatusKeepsTheSourceOfARenameOrCopy(t *testing.T) {
	output := []byte("R  new.go\x00old.go\x00 M edited.go\x00C  copy.go\x00source.go\x00?? fresh.go\x00")

	got := gitrepo.ParseWorktreeStatus(output)

	want := []gitrepo.WorktreeEntry{
		{Code: "R ", Path: "new.go", OldPath: "old.go"},
		{Code: " M", Path: "edited.go"},
		{Code: "C ", Path: "copy.go", OldPath: "source.go"},
		{Code: "??", Path: "fresh.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseWorktreeStatus = %+v, want %+v", got, want)
	}
}
