package main

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// committedPolicy is the policy the analyzed revision holds. The zero value is no policy committed.
type committedPolicy struct {
	path    string
	content json.RawMessage
}

func (c committedPolicy) present() bool { return c.path != "" }

// policyEvidence is everything the choice of policy depends on. The question about a path is a function so that the
// choice stays free of git.
type policyEvidence struct {
	// committed is the policy the revision holds, read as a policy: it is set only where the scan can use it, which is
	// where the working tree selects none or discovery prefers it.
	committed committedPolicy
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

// committedCanonicalOutranks is discovery's order: a committed canonical policy that is missing on disk is not replaced
// by the root project.json that is there.
func committedCanonicalOutranks(committedPath, diskPath string) bool {
	return diskPath == projectconfig.DefaultPath && committedPath == projectconfig.CanonicalPath
}

// choosePolicy applies ADR-009 (#714): the policy on disk is the one used, whatever the revision holds at its path. The
// committed bytes stand in for it where they are the same file, so a clean or line-ending-converted checkout keeps the
// digest CI computes. A committed policy that is missing on disk is used as committed until the scan refuses that state.
func choosePolicy(e policyEvidence) policyChoice {
	if e.disk.Path == "" || committedCanonicalOutranks(e.committed.path, e.disk.Path) {
		return committedOrNone(e.committed)
	}
	cause := relationToHead(e)
	if cause == codesignal.PolicyCauseSameAsHead {
		return usedPolicy(e.disk.Path, e.head.Content, cause)
	}
	return usedPolicy(e.disk.Path, e.disk.Content, cause)
}

// relationToHead is how the policy on disk stands to the copy the revision holds at its path.
func relationToHead(e policyEvidence) codesignal.PolicyCause {
	switch e.head.Kind {
	case projectconfig.CopyAbsent:
		return codesignal.PolicyCauseNotCommitted
	case projectconfig.CopyUnreadable:
		return codesignal.PolicyCauseHeadUnreadable
	case projectconfig.CopyReadable:
		if projectconfig.Digest(e.head.Content) == e.disk.Digest || e.holdsConvertedAt(e.disk.Path) {
			return codesignal.PolicyCauseSameAsHead
		}
	}
	return codesignal.PolicyCauseDiffersFromHead
}

func committedOrNone(committed committedPolicy) policyChoice {
	if committed.present() {
		return usedPolicy(committed.path, committed.content, codesignal.PolicyCauseMissingOnDisk)
	}
	return policyChoice{}
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
