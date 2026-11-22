package gitrepo_test

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestParseIndexTags(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   gitrepo.SparseState
	}{
		{"every entry cached", "H a.go\x00H b.go\x00", gitrepo.SparseState{Tracked: []string{"a.go", "b.go"}}},
		{"an S tag is skip-worktree", "H a.go\x00S b.go\x00", gitrepo.SparseState{Tracked: []string{"a.go", "b.go"}, SkipWorktree: []string{"b.go"}}},
		{"a lowercase tag is assume-unchanged", "H a.go\x00h b.go\x00", gitrepo.SparseState{Tracked: []string{"a.go", "b.go"}, AssumeUnchanged: []string{"b.go"}}},
		{"a path that merely starts with S is not a tag", "H S b.go\x00", gitrepo.SparseState{Tracked: []string{"S b.go"}}},
		{"a path that merely starts with a lowercase letter is not a tag", "H abc.go\x00", gitrepo.SparseState{Tracked: []string{"abc.go"}}},
		{"an assume-unchanged file with a one-character name", "h a\x00", gitrepo.SparseState{Tracked: []string{"a"}, AssumeUnchanged: []string{"a"}}},
		{"a tag and a space with no path is not an entry", "h \x00", gitrepo.SparseState{}},
		{"empty output", "", gitrepo.SparseState{}},
	}
	for _, tt := range tests {
		assertParsedIndexTags(t, tt.name, tt.output, tt.want)
	}
}

func assertParsedIndexTags(t *testing.T, name, output string, want gitrepo.SparseState) {
	t.Helper()
	if got := gitrepo.ParseIndexTags([]byte(output)); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: ParseIndexTags(%q) = %+v, want %+v", name, output, got, want)
	}
}
