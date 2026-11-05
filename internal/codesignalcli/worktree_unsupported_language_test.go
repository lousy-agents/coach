package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestNewUnsupportedPathsAreListedOnceAndHeadsFilesAreNotNew(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{
		{Code: "??", Path: "docs/notes.md"},
		{Code: "A ", Path: "scripts/run.sh"},
		{Code: " A", Path: "scripts/more.sh"},
		{Code: "R ", Path: "scripts/tool.txt", OldPath: "scripts/tool.sh"},
		{Code: "A ", Path: "scripts/run.sh"},
		{Code: "??", Path: "new.go"},
		{Code: "??", Path: "vendor/node_modules/pkg/index.md"},
		{Code: "??", Path: "nested/"},
		{Code: " M", Path: "README.md"},
		{Code: "D ", Path: "project.json"},
		{Code: "??", Path: "project.json"},
	}

	want := []string{"docs/notes.md", "scripts/run.sh", "scripts/more.sh", "scripts/tool.txt"}
	if got := newUnsupportedPaths(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("newUnsupportedPaths = %v, want %v", got, want)
	}
}
