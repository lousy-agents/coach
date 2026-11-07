package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// gitBefore244 is the runner of the real git, run the way a git before 2.44 runs it: Coach's own command, without the
// instruction not to fetch, which such a git does not read, so that what it does with an object it lacks is observed and
// not assumed. The caller's own environment must not carry the instruction either; asGitBefore244 clears it for the test.
func gitBefore244(dir string, args ...string) ([]byte, error) {
	return runBytesBy(commandTold(nil), defaultBudgetFor, dir, args...)
}

// asGitBefore244 clears, for the test, the instruction not to fetch that the caller's environment may carry.
func asGitBefore244(t *testing.T) {
	t.Helper()
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
