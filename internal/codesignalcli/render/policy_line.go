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
	policy := report.Scope.Policy
	if policy == nil {
		return
	}
	origin := effectivePolicyOrigin(policy)
	fmt.Fprintf(b, "policy: %s (%s), %s\n", terminalText(policy.Path), shortPolicyDigest(policy.Digest), policyHeadRelation(origin))
	if warning := policyWarning(policy, origin); warning != "" {
		b.WriteString(warning + "\n")
	}
}

// effectivePolicyOrigin lets MatchesHead decide the cases it can: a report decoded from JSON carries the zero
// Origin, and a true MatchesHead always means the committed policy.
func effectivePolicyOrigin(policy *codesignal.PolicyScope) codesignal.PolicyOrigin {
	if policy.MatchesHead {
		return codesignal.PolicyOriginMatchesHead
	}
	switch policy.Origin {
	case codesignal.PolicyOriginNotCommitted, codesignal.PolicyOriginMissingOnDisk, codesignal.PolicyOriginDiskUnusable:
		return policy.Origin
	}
	return codesignal.PolicyOriginDiffersFromHead
}

func shortPolicyDigest(digest string) string {
	const prefix = "pcfg_"
	if hex, ok := strings.CutPrefix(digest, prefix); ok && len(hex) > policyDigestPrefixHexLength {
		return prefix + hex[:policyDigestPrefixHexLength]
	}
	return digest
}

func policyHeadRelation(origin codesignal.PolicyOrigin) string {
	switch origin {
	case codesignal.PolicyOriginMatchesHead:
		return "same as HEAD."
	case codesignal.PolicyOriginNotCommitted:
		return "not committed at HEAD."
	}
	return "differs from HEAD."
}

func policyWarning(policy *codesignal.PolicyScope, origin codesignal.PolicyOrigin) string {
	switch origin {
	case codesignal.PolicyOriginDiffersFromHead:
		return "WARNING: the project policy on disk differs from HEAD's; this scan used the policy on disk, so its result can differ from CI, which reads the committed policy."
	case codesignal.PolicyOriginNotCommitted:
		return fmt.Sprintf("WARNING: the project policy %s is on disk and not committed at HEAD; this scan used it.", terminalText(policy.Path))
	case codesignal.PolicyOriginMissingOnDisk:
		return fmt.Sprintf("WARNING: %s is committed at HEAD and missing on disk; this scan used the committed copy.", terminalText(policy.Path))
	case codesignal.PolicyOriginDiskUnusable:
		return fmt.Sprintf("WARNING: the project policy on disk cannot be used (%s); this scan used the committed copy.", policy.Reason)
	}
	return ""
}

// policyVerdictClause is the cause a clean result carries when the policy the scan used is not the committed one;
// it is empty when there is no such policy.
func policyVerdictClause(report *codesignal.Report) string {
	policy := report.Scope.Policy
	if policy == nil || policy.MatchesHead {
		return ""
	}
	switch effectivePolicyOrigin(policy) {
	case codesignal.PolicyOriginNotCommitted:
		return "the project policy on disk is not committed at HEAD"
	case codesignal.PolicyOriginMissingOnDisk:
		return "the project policy is missing on disk and the committed copy was used"
	case codesignal.PolicyOriginDiskUnusable:
		return "the project policy on disk cannot be used and the committed copy was used"
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
