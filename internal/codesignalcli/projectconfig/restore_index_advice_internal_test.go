package projectconfig

import "testing"

const (
	notPrintedRestoreFromIndex = "restore it from the index (the name has a character that is not printed here)"
	notPrintedClearMark        = "clear the mark with git update-index --no-skip-worktree (the name has a character that is not printed here)"
)

type adviceCase struct {
	name string
	path string
	want string
}

// refusedPathCases are the paths a printed command must never carry: one that is no single file inside
// the repository would act on every tracked file or on a file outside it, and one with a character a
// terminal would act on is not shown raw.
func refusedPathCases(fallback string) []adviceCase {
	return []adviceCase{
		{"an empty path, which would name every tracked file", "", fallback},
		{"the repository root", ".", fallback},
		{"a path that leaves the repository", "../pol.json", fallback},
		{"an absolute path", "/etc/pol.json", fallback},
		{"a control character", "bad\x1bname.json", fallback},
		{"a newline", "a\nb.json", fallback},
		{"invalid UTF-8", "my\x9bconfig.json", fallback},
	}
}

func runAdviceCases(t *testing.T, name string, advice func(string) string, cases []adviceCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertAdvice(t, name, advice, c) })
	}
}

func assertAdvice(t *testing.T, name string, advice func(string) string, c adviceCase) {
	t.Helper()
	if got := advice(c.path); got != c.want {
		t.Errorf("%s(%q) = %q; want %q", name, c.path, got, c.want)
	}
}

func TestRestoreFromIndexAdvice(t *testing.T) {
	cases := []adviceCase{
		{"a plain name", "pol.json", `restore it with git restore -- ':(top,literal)pol.json'`},
		{"a nested name, from any directory", "policies/p.json", `restore it with git restore -- ':(top,literal)policies/p.json'`},
		{"a single quote", "it's.json", `restore it with git restore -- ':(top,literal)it'\''s.json'`},
		{"command substitution", "$(touch PWNED).json", `restore it with git restore -- ':(top,literal)$(touch PWNED).json'`},
		{"glob characters, which must not be read as a glob", "a[1]*.json", `restore it with git restore -- ':(top,literal)a[1]*.json'`},
		{"spaces", "my policy.json", `restore it with git restore -- ':(top,literal)my policy.json'`},
	}

	runAdviceCases(t, "RestoreFromIndexAdvice", RestoreFromIndexAdvice, append(cases, refusedPathCases(notPrintedRestoreFromIndex)...))
}

func TestClearSkipWorktreeAdvice(t *testing.T) {
	cases := []adviceCase{
		{"a plain name", "pol.json", `clear the mark with git update-index --no-skip-worktree -- 'pol.json' from the repository root`},
		{"a nested name, which is relative to the root and not a pathspec", "policies/p.json", `clear the mark with git update-index --no-skip-worktree -- 'policies/p.json' from the repository root`},
		{"a single quote", "it's.json", `clear the mark with git update-index --no-skip-worktree -- 'it'\''s.json' from the repository root`},
		{"command substitution", "$(touch PWNED).json", `clear the mark with git update-index --no-skip-worktree -- '$(touch PWNED).json' from the repository root`},
		{"glob characters", "a[1]*.json", `clear the mark with git update-index --no-skip-worktree -- 'a[1]*.json' from the repository root`},
		{"spaces", "my policy.json", `clear the mark with git update-index --no-skip-worktree -- 'my policy.json' from the repository root`},
	}

	runAdviceCases(t, "clearSkipWorktreeAdvice", clearSkipWorktreeAdvice, append(cases, refusedPathCases(notPrintedClearMark)...))
}
