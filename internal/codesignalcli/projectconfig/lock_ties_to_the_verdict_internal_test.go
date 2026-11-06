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

// The lock matters only for an offer that prints a restore. A staged edit is explained as a staged edit
// whether or not another git process holds the index, because the restore is withheld for that reason and
// the lock changes nothing about it. Going through judgeRestore, not a hand-built judgment, is what ties
// the lock to the verdict: a lock read for every verdict would tell a user with a staged edit to wait.
func TestALockedIndexDoesNotReplaceTheExplanationOfAStagedEdit(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	writeFile(t, repo, "cfg/p.json", `{"staged":true}`)
	stage(t, repo, "cfg/p.json")
	moveAside(t, repo, "cfg/p.json")
	lockTheIndex(t, repo)

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	if status.PolicyCause != projectreadiness.CauseStagedEdit || !strings.Contains(status.Message, "staged edit") {
		t.Errorf("cause %q, message %q; want the staged edit explained, not the lock", status.PolicyCause, status.Message)
	}
}

// A file in the way of the path is the first thing to move, and the restore would write into it, so it
// is told before the lock even when the index is locked too.
func TestAFileInTheWayIsToldBeforeALockedIndex(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	if err := os.RemoveAll(filepath.Join(repo, "cfg")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "cfg", "a file in the way")
	lockTheIndex(t, repo)

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil || status.PolicyCause != projectreadiness.CauseParentNotDirectory {
		t.Errorf("ExplicitOnDiskStatus = %+v, %v; want the file in the way, cause parent_not_directory", status, err)
	}
}
