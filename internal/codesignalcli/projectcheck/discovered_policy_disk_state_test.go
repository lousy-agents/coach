package projectcheck

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A policy the analyzed revision holds is judged from the revision. What the working tree has at its path is
// the scan's to read, not this check's: a link on disk where a valid policy is committed does not make it a gap.
func TestACommittedDiscoveredPolicyIsJudgedFromTheRevisionWhateverIsOnDisk(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, projectconfig.CanonicalPath, validPolicy)
	onDisk := filepath.Join(repo, projectconfig.CanonicalPath)
	if err := os.Remove(onDisk); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Symlink("elsewhere.json", onDisk); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	check, _, err := checkDiscoveredPolicy(repo, "HEAD", projectconfig.CanonicalPath)

	if err != nil || check.State != projectreadiness.Pass {
		t.Errorf("checkDiscoveredPolicy = %+v, %v; want the committed policy to pass", check, err)
	}
}

// What the index holds for a policy missing from disk decides whether a restore is safe. When git cannot say, the
// check stops with that failure and does not call the policy missing, which would offer to author over a file the
// index may hold.
func TestAPolicyWhoseIndexEntryCannotBeListedStopsTheCheck(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", validPolicy)
	if err := os.Remove(filepath.Join(repo, "cfg", "p.json")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	original := runReadinessGit
	withReadinessGit(t, func(dir string, args ...string) ([]byte, error) {
		if args[0] == "ls-files" && slices.Contains(args, "--stage") {
			return nil, errNoAnswer
		}
		return original(dir, args...)
	})

	check, _, err := checkExplicitPolicy(repo, "HEAD", "cfg/p.json")

	requireOperationalError(t, check, err)
}
