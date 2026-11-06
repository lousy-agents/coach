package gitrepo_test

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestParseStatusWarningsNamesEachDirectoryGitCouldNotOpenAndFailsClosed(t *testing.T) {
	tests := []struct {
		name        string
		stderr      string
		wantDirs    []string
		wantUnrecog bool
	}{
		{"a directory without permission", "warning: could not open directory 'pkg/y/': Permission denied\n", []string{"pkg/y/"}, false},
		{"two directories, each once", "warning: could not open directory 'a/': Permission denied\nwarning: could not open directory 'b/c/': Permission denied\nwarning: could not open directory 'a/': Permission denied\n", []string{"a/", "b/c/"}, false},
		{"another reason", "warning: could not open directory 'pkg/y/': Not a directory\n", []string{"pkg/y/"}, false},
		{"a name with an apostrophe", "warning: could not open directory 'it's/': Permission denied\n", []string{"it's/"}, false},
		{"a name holding the closing quote and colon", "warning: could not open directory 'a': b/': Permission denied\n", []string{"a': b/"}, false},
		{"a tab in the name is escaped, never relayed", "warning: could not open directory 'tab\there/': Permission denied\n", []string{`"tab\there/"`}, false},
		{"bytes that are not UTF-8 are escaped", "warning: could not open directory 'pkg/\xffy/': Permission denied\n", []string{`"pkg/\xffy/"`}, false},
		{"an escape character is escaped", "warning: could not open directory 'pkg/\x1b[31my/': Permission denied\n", []string{`"pkg/\x1b[31my/"`}, false},
		{"a line that is cut by a newline in the name keeps the directory", "warning: could not open directory 'a\nb/': Permission denied\n", []string{unnameable}, false},
		{"a warning about something else is unrecognized, not dropped", "warning: unable to access '/home/u/.config/git/ignore': Permission denied\n", nil, true},
		{"a directory and another warning", "warning: could not open directory 'a/': Permission denied\nwarning: something new\n", []string{"a/"}, true},
		{"no stderr", "", nil, false},
		{"a line that is not a warning", "hint: use git status -uno\n", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertStatusWarnings(t, tt.stderr, tt.wantDirs, tt.wantUnrecog)
		})
	}
}

const unnameable = "a directory whose name cannot be shown"

func assertStatusWarnings(t *testing.T, stderr string, wantDirs []string, wantUnrecognized bool) {
	t.Helper()
	got := gitrepo.ParseStatusWarnings([]byte(stderr))
	if !reflect.DeepEqual(got.UnreadableDirectories, wantDirs) || got.Unrecognized != wantUnrecognized {
		t.Errorf("ParseStatusWarnings(%q) = %+v, want directories %v and unrecognized %v", stderr, got, wantDirs, wantUnrecognized)
	}
	if none := got.None(); none != (len(wantDirs) == 0 && !wantUnrecognized) {
		t.Errorf("None() = %v for %+v", none, got)
	}
}
