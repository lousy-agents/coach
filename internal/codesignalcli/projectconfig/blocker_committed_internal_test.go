package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A path that runs through a file is told to move or remove the file only when nothing commits it. A file the
// analyzed revision commits belongs to the repository: the path is what is wrong, and advice to delete the
// file asks the user to lose work.
func TestAPathThroughAFileIsToldToMoveItOnlyWhenNothingCommitsIt(t *testing.T) {
	cases := map[string]struct {
		commits bool
		want    string
		notWant string
	}{
		"a file the revision commits": {true, "which is a file the analyzed revision commits, so the path is probably wrong", "move or remove it"},
		"a file nothing commits":      {false, "move or remove it", "probably wrong"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheBlockerAdvice(t, c.commits, c.want, c.notWant) })
	}
}

func requireTheBlockerAdvice(t *testing.T, commits bool, want, notWant string) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	if commits {
		gitfixture.CommitFile(t, repo, "cfg", "a committed file")
	} else if err := os.WriteFile(filepath.Join(repo, "cfg"), []byte("untracked"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	status, err := ExplicitOnDiskStatus(runProjectConfigGit, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil || status.Missing != MissingBlockedParent {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a policy blocked by the file", status, err)
	}
	if !strings.Contains(status.Message, want) || strings.Contains(status.Message, notWant) || !strings.Contains(status.Message, `"cfg"`) {
		t.Errorf("message %q; want %q, not %q, and the blocker named", status.Message, want, notWant)
	}
}

// Whether the file in the way is committed is a git question; when git cannot answer, the status is that
// failure and not advice that may tell the user to delete committed work.
func TestAPathThroughAFileWhoseCommitStateCannotBeReadIsAnOperationalFailure(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	if err := os.WriteFile(filepath.Join(repo, "cfg"), []byte("untracked"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	failing := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "ls-tree" || args[0] == "cat-file" {
			return nil, errBoom
		}
		return runProjectConfigGit(dir, args...)
	}

	status, err := ExplicitOnDiskStatus(failing, repo, "HEAD", "cfg/p.json")

	if err == nil {
		t.Errorf("ExplicitOnDiskStatus = %+v, nil; want the failed question returned", status)
	}
}
