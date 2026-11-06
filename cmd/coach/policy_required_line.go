package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// policyRequiredLines is the refusal for compiler setup that is blocked behind a
// policy gap, as lines. When no remedy fixes the gap, the first line is why, in
// the words the scan prints for the same policy, because the refusal itself can
// only say that a new policy would not help.
func policyRequiredLines(prefix, code string, remedy projectreadiness.PolicyRemedy, cause, revision, policyPath string) []string {
	line := policyRequiredLine(prefix, code, remedy, revision, policyPath)
	if _, named := projectreadiness.NextActionForRemedy(remedy); named || cause == "" {
		return []string{line}
	}
	return []string{cause, line}
}

// policyRequiredLine is the refusal for compiler setup that is blocked behind a
// policy gap. It names only the remedy readiness named for that policy, so a
// committed policy is restored rather than authored again.
func policyRequiredLine(prefix, code string, remedy projectreadiness.PolicyRemedy, revision, policyPath string) string {
	line := fmt.Sprintf("%s: a reviewed, committed policy is required before compiler setup (%s)", prefix, code)
	action, named := projectreadiness.NextActionForRemedy(remedy)
	switch {
	case !named:
		return line + "; a new policy cannot fix it, so resolve the policy path first."
	case action == projectreadiness.NextActionRestorePolicy:
		return line + "; it is committed at the analyzed revision (" + action + "), so " + projectconfig.RestoreAdvice(revision, policyPath)
	}
	return line + "; run guided policy authoring first (" + action + ")."
}
