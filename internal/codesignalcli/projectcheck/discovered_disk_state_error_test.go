package projectcheck

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The disk state of a discovered policy is asked of the repository root first. A policy whose root git cannot find
// is not a policy with nothing on disk. This fake answers every other question as "nothing there", so dropping
// the failure would end in a gap that offers a new policy, and only the failure itself can stop the check.
func TestADiscoveredPolicyWhoseRepositoryRootCannotBeFoundStopsTheCheckAndIsNotCalledMissing(t *testing.T) {
	withReadinessGit(t, func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			return nil, errNoAnswer
		case "config":
			return []byte("false\n"), nil
		}
		return nil, nil
	})
	missing := projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing, Remedy: projectreadiness.RemedyAuthor}

	check, _, err := gapForDiscoveredPolicyOnDisk(t.TempDir(), "HEAD", ".lousy-agents/coach.json", missing)

	if err == nil {
		t.Errorf("gapForDiscoveredPolicyOnDisk = %+v, nil; want the failure to find the repository root", check)
	}
}
