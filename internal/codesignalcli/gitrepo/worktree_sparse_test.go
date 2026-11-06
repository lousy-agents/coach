package gitrepo_test

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestHasSkipWorktreeEntry(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{"every entry cached", "H a.go\x00H b.go\x00", false},
		{"one entry skip-worktree", "H a.go\x00S b.go\x00", true},
		{"a path that merely starts with S is not a tag", "H S b.go\x00", false},
		{"empty output", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertHasSkipWorktreeEntry(t, tt.output, tt.want)
		})
	}
}

func assertHasSkipWorktreeEntry(t *testing.T, output string, want bool) {
	t.Helper()
	if got := gitrepo.HasSkipWorktreeEntry([]byte(output)); got != want {
		t.Errorf("HasSkipWorktreeEntry(%q) = %v, want %v", output, got, want)
	}
}

func TestHasAssumeUnchangedEntry(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{"every entry cached", "H a.go\x00H b.go\x00", false},
		{"a lowercase tag is assume-unchanged", "H a.go\x00h b.go\x00", true},
		{"skip-worktree alone is not assume-unchanged", "S a.go\x00H b.go\x00", false},
		{"a path that merely starts with a lowercase letter is not a tag", "H abc.go\x00", false},
		{"an assume-unchanged file with a one-character name", "h a\x00", true},
		{"a tag and a space with no path is not an entry", "h \x00", false},
		{"empty output", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertHasAssumeUnchangedEntry(t, tt.output, tt.want)
		})
	}
}

func assertHasAssumeUnchangedEntry(t *testing.T, output string, want bool) {
	t.Helper()
	if got := gitrepo.HasAssumeUnchangedEntry([]byte(output)); got != want {
		t.Errorf("HasAssumeUnchangedEntry(%q) = %v, want %v", output, got, want)
	}
}
