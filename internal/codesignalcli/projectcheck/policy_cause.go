package projectcheck

import "github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"

// causeFor is the reason a policy gap carries: only a gap that no remedy fixes
// needs one, because the other two name what to do.
func causeFor(remedy projectreadiness.PolicyRemedy, message string) string {
	if _, named := projectreadiness.NextActionForRemedy(remedy); named {
		return ""
	}
	return message
}
