package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func explicitStatusFor(t *testing.T, fake indexFake) *ConfigError {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "other.json", "{}")

	status, err := ExplicitOnDiskStatus(fake.run, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	return status
}

func TestAnUnmergedPolicyNothingCommitsNamesTheConflictAndPrintsNoRestore(t *testing.T) {
	lines := indexLine("100644", committedBlob, "1") + indexLine("100644", otherBlob, "3")

	status := explicitStatusFor(t, indexFake{uncommitted: true, lsFiles: lines})

	if !strings.Contains(status.Message, "merge conflict") {
		t.Errorf("message %q; want it to name the unresolved merge conflict", status.Message)
	}
	if strings.Contains(status.Message, "restore it with") || strings.Contains(status.Message, "staged copy") || strings.Contains(status.Message, "committed at the analyzed revision") {
		t.Errorf("message %q; git restore refuses an unmerged path, and the policy is not committed", status.Message)
	}
}

func TestAPolicyNothingCommitsThatIsStagedAsALinkPrintsNoRestore(t *testing.T) {
	status := explicitStatusFor(t, indexFake{uncommitted: true, lsFiles: indexLine("120000", committedBlob, "0")})

	if !strings.Contains(status.Message, "symbolic link") {
		t.Errorf("message %q; want it to say the staged entry is a symbolic link", status.Message)
	}
	if strings.Contains(status.Message, "restore it with") {
		t.Errorf("message %q; restoring would recreate a link the scan refuses", status.Message)
	}
}

func TestAPolicyNothingCommitsThatIsStagedAsASubmoduleEntryPrintsNoRestore(t *testing.T) {
	status := explicitStatusFor(t, indexFake{uncommitted: true, lsFiles: indexLine("160000", committedBlob, "0")})

	if strings.Contains(status.Message, "restore it with") {
		t.Errorf("message %q; a restore would not produce a regular file, which the scan requires", status.Message)
	}
	if !strings.Contains(status.Message, "not a regular file") {
		t.Errorf("message %q; want it to say the staged entry is not a regular file", status.Message)
	}
}

func TestAPolicyNothingCommitsThatIsStagedAndSkipWorktreeMarkedPrintsNoRestore(t *testing.T) {
	status := explicitStatusFor(t, indexFake{uncommitted: true, lsFiles: skippedIndexLine("100644", committedBlob, "0")})

	if !strings.Contains(status.Message, "skip-worktree") || !strings.Contains(status.Message, "git update-index --no-skip-worktree") {
		t.Errorf("message %q; want it to name the skip-worktree mark and the command that clears it", status.Message)
	}
	if strings.Contains(status.Message, "git restore") {
		t.Errorf("message %q; git restore does not match a skip-worktree path, so no restore command may be printed", status.Message)
	}
}

func TestAPolicyNothingCommitsThatASparseCheckoutLeavesOutPrintsNoRestore(t *testing.T) {
	status := explicitStatusFor(t, indexFake{uncommitted: true, sparse: true, lsFiles: skippedIndexLine("100644", committedBlob, "0")})

	if !strings.Contains(status.Message, "sparse checkout") {
		t.Errorf("message %q; want it to name the sparse checkout", status.Message)
	}
	if strings.Contains(status.Message, "git restore") {
		t.Errorf("message %q; git restore does not match a path the sparse checkout skips, so no restore command may be printed", status.Message)
	}
}
