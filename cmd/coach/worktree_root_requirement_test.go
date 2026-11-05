package main

import "testing"

func TestQuoteForShellKeepsAnyValueAsOneWord(t *testing.T) {
	assertQuotedForShell(t, "/repo/root", `'/repo/root'`)
	assertQuotedForShell(t, "/my repo", `'/my repo'`)
	assertQuotedForShell(t, "it's", `'it'\''s'`)
	assertQuotedForShell(t, "a$b;`c`", "'a$b;`c`'")
	assertQuotedForShell(t, "", `''`)
}

func assertQuotedForShell(t *testing.T, value, want string) {
	t.Helper()
	if got := quoteForShell(value); got != want {
		t.Errorf("quoteForShell(%q) = %s, want %s", value, got, want)
	}
}

func TestArgumentsForShellQuotesOnlyWhatTheShellWouldSplitOrExpand(t *testing.T) {
	got := argumentsForShell([]string{"--base", "HEAD~1", "--format=json", "--project-config", "a b.json", "$HOME", ""})

	want := ` --base HEAD~1 --format=json --project-config 'a b.json' '$HOME' ''`
	if got != want {
		t.Errorf("argumentsForShell = %q, want %q", got, want)
	}
}

// Only a tilde that begins a word is expanded by the shell, so ~x pasted from the
// hint would become another user's home directory, while HEAD~1 stays as typed.
func TestArgumentsForShellQuotesAWordThatBeginsWithATilde(t *testing.T) {
	got := argumentsForShell([]string{"~x", "~", "~/notes", "a~b", "HEAD~1"})

	want := ` '~x' '~' '~/notes' a~b HEAD~1`
	if got != want {
		t.Errorf("argumentsForShell = %q, want %q", got, want)
	}
}
