package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestFileExistsAtRevisionIgnoresTreeEntries(t *testing.T) {
	repo := gitfixture.Init(t)
	if err := os.Mkdir(filepath.Join(repo, "package.json"), 0o755); err != nil {
		t.Fatalf("mkdir package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json", "inner"), []byte("not a blob\n"), 0o644); err != nil {
		t.Fatalf("write inner: %v", err)
	}

	addCmd := exec.Command("git", "add", "package.json")
	addCmd.Dir = repo
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	commitCmd := exec.Command("git", "commit", "-m", "tree named package.json")
	commitCmd.Dir = repo
	commitCmd.Env = gitfixture.CommitEnv
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	revOut, err := revCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	revision := strings.TrimSpace(string(revOut))

	boundedRead := func(dir string, args ...string) ([]byte, error) {
		return RunBytesBounded(dir, 1<<20, 64<<10, 30*time.Second, args...)
	}
	exists, err := FileExistsAtRevision(boundedRead, repo, revision, "package.json")
	if err != nil {
		t.Fatalf("fileExistsAtRevision returned error: %v", err)
	}
	if exists {
		t.Fatal("a tree named package.json must not count as the package.json blob")
	}
}
