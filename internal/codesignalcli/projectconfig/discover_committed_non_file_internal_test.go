package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// Discovery asks what the revision holds at the canonical path before it falls back: a directory or a
// submodule committed there is the policy's path, and the scan refuses it, as readiness does, instead of
// calling it "in the worktree but not committed" and going on without a policy.
func TestDiscoverRefusesACanonicalPathTheRevisionCommitsAsSomethingElse(t *testing.T) {
	rows := map[string]struct {
		commit func(t *testing.T, repo string)
		cause  string
	}{
		"a directory":       {func(t *testing.T, repo string) { gitfixture.CommitFile(t, repo, CanonicalPath+"/inner.json", "{}") }, "directory"},
		"a submodule entry": {func(t *testing.T, repo string) { gitfixture.CommitGitlink(t, repo, CanonicalPath) }, "in_submodule"},
	}
	for name, row := range rows {
		t.Run(name, func(t *testing.T) { requireDiscoveryRefuses(t, row.commit, row.cause) })
	}
}

func requireDiscoveryRefuses(t *testing.T, commit func(t *testing.T, repo string), cause string) {
	t.Helper()
	repo := committedRepository(t)
	commit(t, repo)

	selection, err := Discover(repo, "HEAD")

	var refusal *ConfigError
	if err != nil || !errors.As(selection.Fatal, &refusal) || string(refusal.PolicyCause) != cause {
		t.Fatalf("Discover = %+v, %v; want the canonical path refused with cause %q", selection, err, cause)
	}
	if selection.Path != "" || selection.UncommittedCanonical {
		t.Errorf("selection %+v names a policy or calls the path uncommitted; the revision holds something there", selection)
	}
}

// A git question that cannot be answered stops discovery, as every other git failure in it does: the
// canonical path is not assumed to be absent. Here only the question about the entry fails.
func TestDiscoverFailsClosedWhenItCannotTellWhatTheRevisionHoldsAtTheCanonicalPath(t *testing.T) {
	repo := committedRepository(t)
	original := runProjectConfigGit
	t.Cleanup(func() { runProjectConfigGit = original })
	runProjectConfigGit = onlyTheEntryKindFails(repo)

	if selection, err := Discover(repo, "HEAD"); err == nil {
		t.Errorf("Discover = %+v, nil; want the failed git question", selection)
	}
}

// onlyTheEntryKindFails answers like a repository with a commit and no blob at any path, and fails the one
// question that tells a directory from nothing, for the canonical path alone: the legacy path answers
// that nothing is there, so only the canonical question can stop discovery.
func onlyTheEntryKindFails(repo string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "ls-tree" && slices.Contains(args, "-z") && strings.HasSuffix(args[len(args)-1], CanonicalPath):
			return nil, errBoom
		case args[0] == "rev-parse":
			return []byte(repo + "\n"), nil
		}
		return nil, nil
	}
}
