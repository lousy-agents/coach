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

// What the working tree has at a policy's path decides, not the revision: a link on disk where a valid policy is
// committed is a policy that cannot be used, and the committed copy does not stand in for it.
func TestALinkOnDiskWhereAValidDiscoveredPolicyIsCommittedIsAGapWhateverTheRevisionHolds(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, projectconfig.CanonicalPath, validPolicy)
	onDisk := filepath.Join(repo, projectconfig.CanonicalPath)
	if err := os.Remove(onDisk); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Symlink("elsewhere.json", onDisk); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policyref.Discovered(""))

	if err != nil || judgment.check.Code != projectreadiness.GapPolicyInvalid || judgment.check.PolicyCause != projectreadiness.CauseSymbolicLink {
		t.Errorf("judgePolicy = %+v, %v; want policy_invalid with the cause %q", judgment.check, err, projectreadiness.CauseSymbolicLink)
	}
}

// What the index holds for a policy missing from disk decides whether a restore is safe. When git cannot say, the
// check stops with that failure and does not call the policy missing, which would offer to author over a file the
// index may hold. The fault is a real one: the index file is overwritten with bytes git cannot read, so every
// question about the index fails and the questions about objects and refs do not.
func TestAPolicyWhoseIndexCannotBeReadStopsTheCheck(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", validPolicy)
	if err := os.Remove(filepath.Join(repo, "cfg", "p.json")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not an index"), 0o644); err != nil {
		t.Fatalf("corrupt the index: %v", err)
	}

	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policyref.Named("cfg/p.json"))

	requireOperationalError(t, judgment.check, err)
}
