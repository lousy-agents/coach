package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// policyVerdictClause is the cause a clean result carries when the policy the scan used is not the committed one;
// it is empty when there is no such policy.
func policyVerdictClause(report *codesignal.Report) string {
	policy := report.Scope.Policy
	if policy == nil || policy.MatchesHead {
		return ""
	}
	switch effectivePolicyCause(policy) {
	case codesignal.PolicyCauseNotCommitted:
		return "the project policy on disk is not committed at HEAD"
	case codesignal.PolicyCauseHeadUnreadable:
		return "HEAD's copy of the project policy could not be read"
	}
	return "the project policy on disk differs from HEAD"
}

// policyTrailingClause adds the policy cause after the causes of an incomplete analysis as a clause of its own,
// because a policy difference does not make the analysis incomplete.
func policyTrailingClause(report *codesignal.Report) string {
	if cause := policyVerdictClause(report); cause != "" {
		return ", and " + cause
	}
	return ""
}

// renderAllClear writes a verdict with no other cause, qualified only by the policy when the scan used one that is
// not the committed policy.
func renderAllClear(b *strings.Builder, report *codesignal.Report, lead string) {
	if cause := policyVerdictClause(report); cause != "" {
		fmt.Fprintf(b, "%s, but %s.%s\n", lead, cause, layerReachSentence(report))
		return
	}
	fmt.Fprintf(b, "%s%s.\n", lead, layerReachSuffix(report))
}
