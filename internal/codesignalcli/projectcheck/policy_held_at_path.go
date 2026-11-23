package projectcheck

import (
	"strconv"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// heldAtThePathText says what holds the path while nothing is on disk. Where the index holds the user's own staged
// version, the one restore it offers keeps that version, and none brings back the revision's copy over it.
func heldAtThePathText(policyPath string, cause projectreadiness.PolicyCause) string {
	quotedPolicyPath := strconv.Quote(policyPath)
	switch cause {
	case projectreadiness.CauseStagedEdit, projectreadiness.CauseStagedCopy:
		held := "a staged edit to it"
		if cause == projectreadiness.CauseStagedCopy {
			held = "a staged copy of it"
		}
		return "coach codesignal: " + quotedPolicyPath + " is not on disk, and the index holds " + held + " that a restore from the revision would discard; to keep the staged version, " + projectconfig.RestoreFromIndexAdvice(policyPath) + ", then check again"
	}
	return "coach codesignal: " + quotedPolicyPath + " is not on disk, and the index or the analyzed revision holds an entry for it that a restore from the revision would discard or recreate as something a policy cannot be; resolve it with git, then check again"
}
