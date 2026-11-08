package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// unusablePolicyStep is what to do about a committed policy that exists and cannot be used, in words and with no command:
// guided authoring writes only at a path with nothing at it, and a scan opens no session for a policy that exists, since the
// only session it has ends by asking the user to save a candidate over the file. The --prepare-compiler refusal and the
// scan's own report print this one sentence, so they cannot say two things.
const unusablePolicyStep = "the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one"

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
// committed policy is restored rather than authored again. The restore sentence
// is the one the scan builds for the same policy. A policy that exists and cannot
// be used (policy_invalid, whose remedy is a new policy) is fixed, or moved aside
// and the move committed, because guided authoring writes at a path with nothing at
// it; the line says so in words and names no command.
func policyRequiredLine(prefix, code string, remedy projectreadiness.PolicyRemedy, revision, policyPath string) string {
	line := fmt.Sprintf("%s: a reviewed, committed policy is required before compiler setup (%s)", prefix, code)
	action, named := projectreadiness.NextActionForRemedy(remedy)
	switch {
	case !named:
		return line + "; a new policy cannot fix it, so resolve the policy path first."
	case action == projectreadiness.NextActionRestorePolicy:
		return line + projectconfig.RestoreSentence(remedy, revision, policyPath, projectconfig.AsARefusal(action))
	case code == projectreadiness.GapPolicyInvalid:
		return line + "; " + unusablePolicyStep + " (" + action + ")."
	}
	return line + "; run guided policy authoring first (" + action + ")."
}
