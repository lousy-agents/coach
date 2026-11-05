package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// policyRequiredLine is the refusal for compiler setup that is blocked behind a
// policy gap. It names only the remedy readiness named for that policy, so a
// committed policy is restored rather than authored again.
func policyRequiredLine(prefix, code string, remedy projectreadiness.PolicyRemedy, revision, policyPath string) string {
	line := fmt.Sprintf("%s: a reviewed, committed policy is required before compiler setup (%s)", prefix, code)
	switch remedy {
	case projectreadiness.RemedyRestore:
		return line + "; it is committed at the analyzed revision (" + projectreadiness.NextActionRestorePolicy + "), so " + projectconfig.RestoreAdvice(revision, policyPath)
	case projectreadiness.RemedyAuthor:
		return line + "; run guided policy authoring first (" + projectreadiness.NextActionAuthorPolicy + ")."
	}
	return line + "; a new policy cannot fix it, so resolve the policy path first."
}
