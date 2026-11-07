package projectcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A policy Coach would discover that is on disk and not committed is not a gap a new policy fixes: authoring
// refuses a target that exists, and the scan says the file was not read because it is not committed. The check
// says the same and names the same way out as for a policy named with --project-config: commit it.
func TestADiscoveredPolicyOnDiskThatIsNotCommittedIsNotAGapANewPolicyFixes(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	writeWorktreeFile(t, repo, projectconfig.CanonicalPath, `{"mine":true}`)

	check, _, err := checkDiscoveredPolicy(repo, "HEAD", projectconfig.CanonicalPath)

	if err != nil || check.Code != projectreadiness.GapPolicyMissing {
		t.Fatalf("checkDiscoveredPolicy = %+v, %v; want a policy_missing gap", check, err)
	}
	if check.Remedy != projectreadiness.RemedyNone || check.PolicyCause != projectreadiness.CauseNotCommitted {
		t.Errorf("remedy %d, cause %q; want no remedy and the cause %q", check.Remedy, check.PolicyCause, projectreadiness.CauseNotCommitted)
	}
}

func TestADiscoveredPolicyWithNothingOnDiskIsStillAGapANewPolicyFixes(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")

	check, _, err := checkDiscoveredPolicy(repo, "HEAD", projectconfig.CanonicalPath)

	if err != nil || check.Code != projectreadiness.GapPolicyMissing || check.Remedy != projectreadiness.RemedyAuthor || check.PolicyCause != "" {
		t.Errorf("checkDiscoveredPolicy = %+v, %v; want a policy_missing gap that offers a new policy and names no cause", check, err)
	}
}

// A file where the policy's directory should be cannot be written around by authoring either, and says so.
func TestADiscoveredPolicyUnderAFileOnDiskNamesTheFileInTheWay(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	writeWorktreeFile(t, repo, ".lousy-agents", "a file in the way")

	check, _, err := checkDiscoveredPolicy(repo, "HEAD", projectconfig.CanonicalPath)

	if err != nil || check.Remedy != projectreadiness.RemedyNone || check.PolicyCause != projectreadiness.CauseParentNotDirectory {
		t.Errorf("checkDiscoveredPolicy = %+v, %v; want no remedy and the cause %q", check, err, projectreadiness.CauseParentNotDirectory)
	}
}

// A question git cannot answer about the disk stops the check; it is not turned into a gap.
func TestADiscoveredPolicyWhoseDiskStateCannotBeReadStopsTheCheck(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	withReadinessGit(t, questionsWhereOnly(theRepositoryRootLookup))

	check, _, err := gapForDiscoveredPolicyOnDisk(repo, "HEAD", projectconfig.CanonicalPath, projectreadiness.Check{Code: projectreadiness.GapPolicyMissing})

	if err == nil {
		t.Errorf("gapForDiscoveredPolicyOnDisk = %+v, nil; want the failed question returned", check)
	}
}

func writeWorktreeFile(t *testing.T, repo, path, content string) {
	t.Helper()
	full := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
