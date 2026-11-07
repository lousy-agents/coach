package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// policyOf is a valid policy padded with blanks to exactly size bytes.
func policyOf(size int) string {
	const policy = `{"schema_version":"1","roots":["."]}`
	return policy + strings.Repeat(" ", size-len(policy))
}

// The readiness read holds the committed policy to the same size as the scan does: a document of exactly MaxBytes is read, and
// one byte more is refused as a policy that is over the size budget, a gap the author can fix and not a failure of git.
func TestLoadForReadinessReadsAPolicyOfExactlyTheSizeBudgetAndRefusesOneByteMore(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, "cfg/at.json", policyOf(MaxBytes))
	gitfixture.CommitFile(t, repo, "cfg/over.json", policyOf(MaxBytes+1))

	atBudget, atErr := LoadForReadiness(repo, "HEAD", "cfg/at.json")
	_, overErr := LoadForReadiness(repo, "HEAD", "cfg/over.json")

	if atErr != nil || len(atBudget.Roots) != 1 {
		t.Errorf("LoadForReadiness of %d bytes = %+v, %v; want the policy read: the budget is on what is over it", MaxBytes, atBudget, atErr)
	}
	var refusal *ConfigError
	if !errors.As(overErr, &refusal) || !strings.Contains(refusal.Message, "size budget") {
		t.Errorf("LoadForReadiness of %d bytes = %v; want a ConfigError that says the policy is over the size budget", MaxBytes+1, overErr)
	}
}
