package gitrepo_test

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestParseGitlinksKeepsOnlyTheSubmoduleEntries(t *testing.T) {
	output := "100644 aaa 0\tkeep.go\x00160000 bbb 0\tthird/lib\x00100755 ccc 0\tbin/run\x00160000 ddd 0\tvendor/sub\x00160000 eee 1\tvendor/sub\x00"

	got := gitrepo.ParseGitlinks([]byte(output))

	want := []string{"third/lib", "vendor/sub"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseGitlinks = %v, want %v", got, want)
	}
}
