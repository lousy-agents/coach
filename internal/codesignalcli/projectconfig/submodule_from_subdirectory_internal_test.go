package projectconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// recordedAsSubmodule asks about a path relative to the repository root, so the
// answer must not depend on the directory the scan starts in. git prints index paths
// relative to the directory it runs in unless it is told otherwise, and a name that
// does not equal the one asked about reads as "not a submodule", which sends the user
// back to committing a file this repository cannot commit.
func TestARecordedSubmoduleIsFoundFromASubdirectoryOfTheRepository(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	head, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "update-index", "--add", "--cacheinfo", "160000,"+string(head[:40])+",vendor/sub").Run(); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"", "vendor", "other"} {
		t.Run("from "+directory, func(t *testing.T) { requireTheSubmoduleIsFoundFrom(t, repo, directory) })
	}
}

func requireTheSubmoduleIsFoundFrom(t *testing.T, repo, directory string) {
	t.Helper()
	dir := filepath.Join(repo, directory)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if found, err := submoduleRecordedAs(NewGitRunner(), dir, "vendor/sub"); err != nil || found != "vendor/sub" {
		t.Errorf("submoduleRecordedAs(%q, vendor/sub) = %q, %v; the index holds a submodule there whatever directory git runs in", dir, found, err)
	}
}
