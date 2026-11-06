package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func statusFor(t *testing.T, fake indexFake) *ConfigError {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	moveAside(t, repo, "cfg/p.json")
	status, err := ExplicitOnDiskStatus(fake.run, repo, "HEAD", "cfg/p.json")
	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	return status
}

func TestAManualSkipWorktreeBitIsNotCalledASparseCheckout(t *testing.T) {
	status := statusFor(t, indexFake{lsFiles: skippedIndexLine("100644", committedBlob, "0")})

	if strings.Contains(status.Message, "sparse checkout") {
		t.Errorf("message %q; a skip-worktree bit set by hand is not a sparse checkout, and widening one would not help", status.Message)
	}
	if !strings.Contains(status.Message, "skip-worktree") || !strings.Contains(status.Message, "git update-index --no-skip-worktree") {
		t.Errorf("message %q; want it to name the skip-worktree bit and the command that clears it", status.Message)
	}
}

func TestASparseCheckoutStillNamesTheSparseCheckout(t *testing.T) {
	status := statusFor(t, indexFake{lsFiles: skippedIndexLine("100644", committedBlob, "0"), sparse: true})

	if !strings.Contains(status.Message, "sparse checkout") {
		t.Errorf("message %q; want a skipped path under core.sparseCheckout to name the sparse checkout", status.Message)
	}
}

func TestUnmergedStagesAreAMergeConflictNotAStagedEdit(t *testing.T) {
	fake := indexFake{lsFiles: indexLine("100644", committedBlob, "1") + indexLine("100644", otherBlob, "2") + indexLine("100644", committedBlob, "3")}

	status := statusFor(t, fake)

	if !strings.Contains(status.Message, "unresolved merge conflict") {
		t.Errorf("message %q; want it to name the unresolved merge conflict", status.Message)
	}
	if strings.Contains(status.Message, "git diff --cached") || strings.Contains(status.Message, "staged edit") {
		t.Errorf("message %q; an unmerged path has no staged edit to review with git diff --cached", status.Message)
	}
}

func TestAPolicyStagedButNeverCommittedIsOfferedFromTheIndex(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "other.json", "{}")

	status, err := ExplicitOnDiskStatus(indexFake{uncommitted: true, lsFiles: indexLine("100644", committedBlob, "0")}.run, repo, "HEAD", "new.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	if status.Remedy == projectreadiness.RemedyAuthor || strings.Contains(status.Message, "--suggest-project-config") {
		t.Errorf("remedy %v, message %q; a staged copy exists, so a new policy must not be offered", status.Remedy, status.Message)
	}
	if !strings.Contains(status.Message, "staged copy") || !strings.Contains(status.Message, "git restore --") {
		t.Errorf("message %q; want it to say the index holds a copy and how to restore it", status.Message)
	}
}
