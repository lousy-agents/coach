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
