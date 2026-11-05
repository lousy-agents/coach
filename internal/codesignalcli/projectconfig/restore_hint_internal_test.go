package projectconfig

import "testing"

func TestRestoreCommand(t *testing.T) {
	cases := []struct {
		name     string
		revision string
		path     string
		want     string
		offered  bool
	}{
		{"plain", "abc123", "pol.json", `git restore --source=abc123 --staged --worktree -- ':(top,literal)pol.json'`, true},
		{"nested", "abc123", "policies/p.json", `git restore --source=abc123 --staged --worktree -- ':(top,literal)policies/p.json'`, true},
		{"a single quote", "abc123", "it's.json", `git restore --source=abc123 --staged --worktree -- ':(top,literal)it'\''s.json'`, true},
		{"command substitution", "abc123", "$(touch PWNED).json", `git restore --source=abc123 --staged --worktree -- ':(top,literal)$(touch PWNED).json'`, true},
		{"glob characters", "abc123", "a[1]*.json", `git restore --source=abc123 --staged --worktree -- ':(top,literal)a[1]*.json'`, true},
		{"spaces", "abc123", "my policy.json", `git restore --source=abc123 --staged --worktree -- ':(top,literal)my policy.json'`, true},
		{"a revision that needs quoting", "main; id", "pol.json", `git restore --source='main; id' --staged --worktree -- ':(top,literal)pol.json'`, true},
		{"a control character", "abc123", "bad\x1bname.json", "", false},
		{"a newline", "abc123", "a\nb.json", "", false},
		{"invalid UTF-8", "abc123", "my\x9bconfig.json", "", false},
		{"a revision with a control character", "a\x1bb", "pol.json", "", false},
		{"a revision that is not valid UTF-8", "a\x9bb", "pol.json", "", false},
		{"an empty path, which would restore every tracked file", "abc123", "", "", false},
		{"the repository root", "abc123", ".", "", false},
		{"a path that leaves the repository", "abc123", "../pol.json", "", false},
		{"an absolute path", "abc123", "/etc/pol.json", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRestoreCommand(t, c.revision, c.path, c.want, c.offered)
		})
	}
}

func assertRestoreCommand(t *testing.T, revision, path, want string, wantOffered bool) {
	t.Helper()
	got, offered := RestoreCommand(revision, path)
	if offered != wantOffered || got != want {
		t.Errorf("RestoreCommand(%q, %q) = %q, %v; want %q, %v", revision, path, got, offered, want, wantOffered)
	}
}

func TestRestoreAdvice(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"a name that can be printed", "pol.json", `restore it with git restore --source=abc123 --staged --worktree -- ':(top,literal)pol.json'`},
		{"a name that cannot be printed", "bad\x1bname.json", "restore it from git at the analyzed revision (the name has a character that is not printed here)"},
		{"a path that is not one file", "", "restore it from git at the analyzed revision (the name has a character that is not printed here)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRestoreAdvice(t, c.path, c.want)
		})
	}
}

func assertRestoreAdvice(t *testing.T, path, want string) {
	t.Helper()
	if got := RestoreAdvice("abc123", path); got != want {
		t.Errorf("RestoreAdvice = %q; want %q", got, want)
	}
}
