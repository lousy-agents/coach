package projectcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A policy Coach would discover that is on disk and not committed is the policy: the check passes it with the roots of
// the file, and says that the revision holds nothing for it.
func TestADiscoveredPolicyOnDiskThatIsNotCommittedPassesAndSaysSo(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	writeWorktreeFile(t, repo, projectconfig.CanonicalPath, `{"schema_version":"1","roots":["src"]}`)

	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policyref.Discovered(""))

	if err != nil || judgment.check.State != projectreadiness.Pass || len(judgment.roots) != 1 || judgment.roots[0] != "src" {
		t.Fatalf("judgePolicy = %+v, roots %v, %v; want a pass with the roots of the file on disk", judgment.check, judgment.roots, err)
	}
	if len(judgment.warnings) != 1 || judgment.warnings[0].Code != projectreadiness.WarnPolicyNotCommitted {
		t.Errorf("warnings = %+v; want %q", judgment.warnings, projectreadiness.WarnPolicyNotCommitted)
	}
}

func TestADiscoveredPolicyWithNothingOnDiskIsStillAGapANewPolicyFixes(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")

	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policyref.Discovered(""))

	check := judgment.check
	if err != nil || check.Code != projectreadiness.GapPolicyMissing || check.Remedy != projectreadiness.RemedyAuthor || check.PolicyCause != "" {
		t.Errorf("judgePolicy = %+v, %v; want a policy_missing gap that offers a new policy and names no cause", check, err)
	}
}

// A file where the policy's directory should be cannot be written around by authoring either, and says so.
func TestADiscoveredPolicyUnderAFileOnDiskNamesTheFileInTheWay(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	writeWorktreeFile(t, repo, ".lousy-agents", "a file in the way")

	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policyref.Discovered(""))

	check := judgment.check
	if err != nil || check.Remedy != projectreadiness.RemedyNone || check.PolicyCause != projectreadiness.CauseParentNotDirectory {
		t.Errorf("judgePolicy = %+v, %v; want no remedy and the cause %q", check, err, projectreadiness.CauseParentNotDirectory)
	}
}

// A question git cannot answer about the disk stops the check; it is not turned into a gap.
func TestADiscoveredPolicyWhoseDiskStateCannotBeReadStopsTheCheck(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	run := questionsWhereOnly(t, theRepositoryRootLookup)

	judgment, err := judgePolicy(run, repo, "HEAD", policyref.Discovered(""))

	if err == nil {
		t.Errorf("judgePolicy = %+v, nil; want the failed question returned", judgment.check)
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
