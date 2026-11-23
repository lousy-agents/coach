package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// unusablePolicyStep is what to do about a policy file that exists and cannot be used, in words and with no command:
// guided authoring writes only at a path with nothing at it, and a scan opens no session for a policy that exists, since the
// only session it has ends by asking the user to save a candidate over the file. The --prepare-compiler refusal and the
// scan's own report print this one sentence, so they cannot say two things.
const unusablePolicyStep = "the policy on disk cannot be used: fix the file"

// policyRequiredLines is the refusal for compiler setup that is blocked behind a
// policy gap, as lines. When no remedy fixes the gap, the first line is why, in
// the words the scan prints for the same policy, because the refusal itself can
// only say that a new policy would not help.
func policyRequiredLines(prefix, code string, remedy projectreadiness.PolicyRemedy, cause projectreadiness.PolicyCause, causeText, revision, policyPath string) []string {
	line := policyRequiredLine(prefix, code, remedy, cause, revision, policyPath)
	if _, named := projectreadiness.NextActionForRemedy(remedy); named || causeText == "" {
		return []string{line}
	}
	return []string{causeText, line}
}

// fileOnDiskIsTheProblem is true for the causes of a policy that is a regular file on disk whose content or read failed; a
// policy path that holds a directory, a link or a file in a directory's place is resolved at the path instead.
func fileOnDiskIsTheProblem(cause projectreadiness.PolicyCause) bool {
	return cause == "" || cause == projectreadiness.CauseUndetermined || cause == projectreadiness.CauseUnreadableOnDisk
}

// policyRequiredLine is the refusal for compiler setup that is blocked behind a
// policy gap. It names only the remedy readiness named for that policy, so a
// committed policy is restored rather than authored again. The restore sentence
// is the one the scan builds for the same policy. A policy that exists and cannot
// be used (policy_invalid, whose remedy is a new policy) is fixed in place, because
// guided authoring writes at a path with nothing at it; the line says so in words and
// names no command.
func policyRequiredLine(prefix, code string, remedy projectreadiness.PolicyRemedy, cause projectreadiness.PolicyCause, revision, policyPath string) string {
	line := fmt.Sprintf("%s: a project policy that passes --check-project is required before compiler setup (%s)", prefix, code)
	action, named := projectreadiness.NextActionForRemedy(remedy)
	switch {
	case !named && code == projectreadiness.GapPolicyInvalid && fileOnDiskIsTheProblem(cause):
		return line + "; " + unusablePolicyStep + "."
	case !named:
		return line + "; a new policy cannot fix it, so resolve the policy path first."
	case action == projectreadiness.NextActionRestorePolicy:
		return line + projectconfig.RestoreSentence(remedy, revision, policyPath, projectconfig.AsARefusal(action))
	case code == projectreadiness.GapPolicyInvalid:
		return line + "; " + unusablePolicyStep + " (" + action + ")."
	}
	return line + "; run guided policy authoring first (" + action + ")."
}
