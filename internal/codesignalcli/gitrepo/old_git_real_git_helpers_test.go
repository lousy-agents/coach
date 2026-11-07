package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// asGitBefore244 makes every git this package starts run the way a git before 2.44 runs it: it does not read the
// instruction not to fetch. The git is the real one and the command is Coach's own; only the instruction is left
// out, through lazyFetchInstruction, so what such a git does with an object it lacks is observed and not assumed.
// The caller's own environment must not carry the instruction either.
func asGitBefore244(t *testing.T) {
	t.Helper()
	original := lazyFetchInstruction
	lazyFetchInstruction = nil
	t.Cleanup(func() { lazyFetchInstruction = original })
	if inherited, set := os.LookupEnv("GIT_NO_LAZY_FETCH"); set {
		t.Cleanup(func() { _ = os.Setenv("GIT_NO_LAZY_FETCH", inherited) })
		_ = os.Unsetenv("GIT_NO_LAZY_FETCH")
	}
}

// objectsIn counts the loose and packed object files of a repository, to show that a run wrote none.
func objectsIn(t *testing.T, repository string) int {
	t.Helper()
	count := 0
	_ = filepath.WalkDir(filepath.Join(repository, ".git", "objects"), func(_ string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// commitOnlyTheRemoteHas adds a commit to a bare repository that no clone of it has fetched, and returns its id: the
// shape of a --base that names a pull request's head, or a branch the checkout did not bring.
func commitOnlyTheRemoteHas(t *testing.T, bare string) string {
	t.Helper()
	tree := strings.TrimSpace(gitfixture.Git(t, bare, "rev-parse", "HEAD^{tree}"))
	parent := strings.TrimSpace(gitfixture.Git(t, bare, "rev-parse", "HEAD"))
	return strings.TrimSpace(gitfixture.Git(t, bare, "commit-tree", "-p", parent, "-m", "only the remote has this", tree))
}
