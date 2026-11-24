package main

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// policyEvidence is everything the choice of policy depends on. The question about a path is a function so that the
// choice stays free of git.
type policyEvidence struct {
	// headPath is the path of the policy the revision selects, empty where it selects none.
	headPath string
	// head is what the revision holds at the path of the policy on disk, read for comparison only.
	head             projectconfig.CommittedCopy
	disk             projectconfig.DiskSelection
	holdsConvertedAt func(path string) bool
}

// policyChoice is the policy a scan uses. An empty scope.Path means the scan uses none.
type policyChoice struct {
	content json.RawMessage
	scope   codesignal.PolicyScope
}

// choosePolicy applies ADR-009 (#714): the policy on disk is the one used, whatever the revision holds at its path, and
// a scan whose working tree selects none uses none: no committed copy stands in for a policy that is missing on disk.
// The committed bytes stand in for the policy on disk where they are the same file, so a clean or line-ending-converted
// checkout keeps the digest CI computes.
func choosePolicy(e policyEvidence) policyChoice {
	if e.disk.Path == "" {
		return policyChoice{}
	}
	cause := relationToHead(e)
	if cause == codesignal.PolicyCauseSameAsHead {
		return usedPolicy(e.disk.Path, e.head.Content, cause)
	}
	choice := usedPolicy(e.disk.Path, e.disk.Content, cause)
	choice.scope.HeadBound = headBoundOf(e.head.Bound)
	return choice
}

// headBoundOf is the bound of git the text report names where HEAD's copy could not be read.
func headBoundOf(bound projectconfig.CopyBound) codesignal.HeadBound {
	switch bound {
	case projectconfig.CopyBoundTime:
		return codesignal.HeadBoundTime
	case projectconfig.CopyBoundStderr:
		return codesignal.HeadBoundStderr
	}
	return codesignal.HeadBoundNone
}

// relationToHead is how the policy on disk stands to the policy the revision selects. A file the revision commits at
// another path than the one it selects is a different policy whatever its bytes, so only the selected path can match.
func relationToHead(e policyEvidence) codesignal.PolicyCause {
	switch e.head.Kind {
	case projectconfig.CopyAbsent:
		return codesignal.PolicyCauseNotCommitted
	case projectconfig.CopyUnreadable:
		return codesignal.PolicyCauseHeadUnreadable
	case projectconfig.CopyReadable:
		if e.headPath == e.disk.Path && (projectconfig.Digest(e.head.Content) == e.disk.Digest || e.holdsConvertedAt(e.disk.Path)) {
			return codesignal.PolicyCauseSameAsHead
		}
	}
	return codesignal.PolicyCauseDiffersFromHead
}

func usedPolicy(path string, content json.RawMessage, cause codesignal.PolicyCause) policyChoice {
	return policyChoice{
		content: content,
		scope: codesignal.PolicyScope{
			Path:        path,
			Digest:      projectconfig.Digest(content),
			MatchesHead: cause == codesignal.PolicyCauseSameAsHead,
			Cause:       cause,
		},
	}
}
