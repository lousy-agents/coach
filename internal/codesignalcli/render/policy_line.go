package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const policyDigestPrefixHexLength = 12

// renderPolicyDisclosure says, directly under the scan's summary line, which project policy the scan used and,
// when that policy is not the committed one at HEAD, warns that the result can differ from CI.
func renderPolicyDisclosure(b *strings.Builder, report *codesignal.Report) {
	absent, hasAbsent := codesignal.DiagnosticOfKind(report.Diagnostics, codesignal.DiagKindProjectPolicyAbsentOnDisk)
	policy := report.Scope.Policy
	if policy != nil {
		cause := effectivePolicyCause(policy)
		fmt.Fprintf(b, "policy: %s (%s), %s\n", terminalText(policy.Path), shortPolicyDigest(policy.Digest), policyHeadRelation(cause))
		if warning := policyWarning(policy, cause); warning != "" && !(hasAbsent && cause == codesignal.PolicyCauseDiffersFromHead) {
			b.WriteString(warning + "\n")
		}
	}
	if hasAbsent {
		b.WriteString(absentPolicyWarning(absent.Path, policy) + "\n")
	}
}

// effectivePolicyCause lets MatchesHead decide the cases it can: a report decoded from JSON carries the zero
// Cause, and a true MatchesHead always means the committed policy.
func effectivePolicyCause(policy *codesignal.PolicyScope) codesignal.PolicyCause {
	if policy.MatchesHead {
		return codesignal.PolicyCauseSameAsHead
	}
	switch policy.Cause {
	case codesignal.PolicyCauseNotCommitted, codesignal.PolicyCauseHeadUnreadable:
		return policy.Cause
	}
	return codesignal.PolicyCauseDiffersFromHead
}

func shortPolicyDigest(digest string) string {
	const prefix = "pcfg_"
	if hex, ok := strings.CutPrefix(digest, prefix); ok && len(hex) > policyDigestPrefixHexLength {
		return prefix + hex[:policyDigestPrefixHexLength]
	}
	return digest
}

func policyHeadRelation(cause codesignal.PolicyCause) string {
	switch cause {
	case codesignal.PolicyCauseSameAsHead:
		return "same as HEAD."
	case codesignal.PolicyCauseNotCommitted:
		return "not committed at HEAD."
	case codesignal.PolicyCauseHeadUnreadable:
		return "not compared with HEAD."
	}
	return "differs from HEAD."
}

func policyWarning(policy *codesignal.PolicyScope, cause codesignal.PolicyCause) string {
	switch cause {
	case codesignal.PolicyCauseDiffersFromHead:
		return "WARNING: the project policy on disk differs from HEAD's; this scan used the policy on disk, so its result can differ from CI, which reads the committed policy."
	case codesignal.PolicyCauseNotCommitted:
		return fmt.Sprintf("WARNING: the project policy %s is on disk and not committed at HEAD; this scan used it.", terminalText(policy.Path))
	case codesignal.PolicyCauseHeadUnreadable:
		return fmt.Sprintf("WARNING: HEAD's copy of the project policy %s could not be read%s; this scan used the policy on disk, so its result can differ from CI, which reads the committed policy.", terminalText(policy.Path), headBoundClause(policy.HeadBound))
	}
	return ""
}

// headBoundClause names the bound of git that stopped the read of HEAD's copy, and nothing for a copy no bound stopped.
func headBoundClause(bound codesignal.HeadBound) string {
	switch bound {
	case codesignal.HeadBoundTime:
		return " within git's time bound"
	case codesignal.HeadBoundStderr:
		return " within git's stderr bound"
	}
	return ""
}
