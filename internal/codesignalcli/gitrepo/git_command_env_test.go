package gitrepo

import (
	"context"
	"strings"
	"testing"
)

// The last value of a key wins when an environment repeats it, which is what exec does with cmd.Env.
func effectiveValue(env []string, key string) string {
	value := ""
	for _, entry := range env {
		if rest, ok := strings.CutPrefix(entry, key+"="); ok {
			value = rest
		}
	}
	return value
}

func TestGitCommandKeepsGitFromFetchingAskingOrSmudgingWhateverTheCallerSet(t *testing.T) {
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GIT_LFS_SKIP_SMUDGE", "0")

	env := gitCommand(context.Background(), "repo", "status").Env

	if got := effectiveValue(env, "GIT_NO_LAZY_FETCH"); got != "1" {
		t.Errorf("GIT_NO_LAZY_FETCH = %q; want 1, so a missing object is never fetched", got)
	}
	if got := effectiveValue(env, "GIT_TERMINAL_PROMPT"); got != "0" {
		t.Errorf("GIT_TERMINAL_PROMPT = %q; want 0, so git never asks on the terminal", got)
	}
	if got := effectiveValue(env, "GIT_LFS_SKIP_SMUDGE"); got != "1" {
		t.Errorf("GIT_LFS_SKIP_SMUDGE = %q; want 1, so git-lfs leaves a pointer file as it is", got)
	}
}
