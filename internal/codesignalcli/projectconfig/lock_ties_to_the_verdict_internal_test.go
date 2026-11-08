package projectconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func stage(t *testing.T, repo, name string) {
	t.Helper()
	if err := exec.Command("git", "-C", repo, "add", name).Run(); err != nil {
		t.Fatal(err)
	}
}

func lockTheIndex(t *testing.T, repo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The lock matters for an offer that prints a restore, and a staged edit's offer prints one: the command that
// keeps the edit is git restore from the index, which takes the index lock and fails while another git process
// holds it. So the lock is told first, and the staged edit is explained once it is gone. Going through
// judgeRestore, not a hand-built judgment, is what ties the lock to the verdict: a lock read for every verdict
// would tell a user whose offer prints no command to wait.
func TestALockedIndexIsToldBeforeAStagedEditBecauseTheCommandThatKeepsItWouldFail(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", validPolicy)
	writeFile(t, repo, "cfg/p.json", `{"staged":true}`)
	stage(t, repo, "cfg/p.json")
	moveAside(t, repo, "cfg/p.json")
	lockTheIndex(t, repo)

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	if status.PolicyCause != projectreadiness.CauseIndexLocked || strings.Contains(status.Message, "git restore") {
		t.Errorf("cause %q, message %q; want the lock told and no command that would fail", status.PolicyCause, status.Message)
	}
}

// A file in the way of the path is the first thing to move, and the restore would write into it, so it
// is told before the lock even when the index is locked too.
func TestAFileInTheWayIsToldBeforeALockedIndex(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", validPolicy)
	if err := os.RemoveAll(filepath.Join(repo, "cfg")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "cfg", "a file in the way")
	lockTheIndex(t, repo)

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil || status.PolicyCause != projectreadiness.PolicyCause("parent_not_directory") {
		t.Errorf("ExplicitOnDiskStatus = %+v, %v; want the file in the way, cause parent_not_directory", status, err)
	}
}
