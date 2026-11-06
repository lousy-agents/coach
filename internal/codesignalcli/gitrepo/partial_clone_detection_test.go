package gitrepo

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestIsPartialCloneReadsEitherMarkerGitWrites(t *testing.T) {
	cases := []struct {
		name          string
		configuration string
		want          bool
	}{
		{"a promisor remote", promisorOrigin, true},
		{"a promisor remote spelled yes", "remote.origin.promisor yes\n", true},
		{"a promisor remote set to false", "remote.origin.promisor false\n", false},
		{"the extension older git wrote", "extensions.partialclone origin\n", true},
		{"no marker at all", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPartialCloneRead(t, c.configuration, c.want)
		})
	}
}

func assertPartialCloneRead(t *testing.T, configuration string, want bool) {
	t.Helper()
	if got := IsPartialClone(gitSaying("", configuration, nil), "repo"); got != want {
		t.Errorf("IsPartialClone = %v; want %v", got, want)
	}
}

func TestIsPartialCloneSeesTheConfigurationGitWritesForAFilteredClone(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	clone, _ := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })
	ordinary := gitfixture.Init(t)

	if !IsPartialClone(RunBytes, clone) {
		t.Errorf("IsPartialClone(%s) = false; want true for a clone made with --filter", clone)
	}
	if IsPartialClone(RunBytes, ordinary) {
		t.Errorf("IsPartialClone(%s) = true; want false for an ordinary repository", ordinary)
	}
}
