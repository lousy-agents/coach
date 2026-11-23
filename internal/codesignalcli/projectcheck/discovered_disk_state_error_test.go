package projectcheck

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// The working tree of a policy is asked of the repository root first. A policy whose root git cannot find is not a
// policy with nothing on disk. This fake answers every other question as "nothing there", so dropping the failure would
// end in a gap that offers a new policy, and only the failure itself can stop the check.
func TestAPolicyWhoseRepositoryRootCannotBeFoundStopsTheCheckAndIsNotCalledMissing(t *testing.T) {
	run := func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			return nil, errNoAnswer
		case "config":
			return []byte("false\n"), nil
		}
		return nil, nil
	}
	for name, policy := range map[string]policyref.Ref{"discovered": policyref.Discovered(""), "named": policyref.Named("cfg/p.json")} {
		t.Run(name, func(t *testing.T) {
			judgment, err := judgePolicy(run, t.TempDir(), "HEAD", policy)

			if err == nil {
				t.Errorf("judgePolicy = %+v, nil; want the failure to find the repository root", judgment.check)
			}
		})
	}
}
