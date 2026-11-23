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
	committed        committedPolicy
	disk             projectconfig.DiskSelection
	holdsConvertedAt func(path string) bool
}

// policyChoice is the policy a scan uses. An empty scope.Path means the scan uses none.
type policyChoice struct {
	content json.RawMessage
	scope   codesignal.PolicyScope
}

// choosePolicy applies ADR-009 (#714): the policy on disk is the one used. The committed bytes stand in for it where they
// are the same file, so a clean or line-ending-converted checkout keeps the digest CI computes. A committed policy that
// is missing on disk is used as committed until the scan refuses that state. Discovery keeps its order: a committed
// canonical policy that is missing on disk is not replaced by the root project.json that is there.
func choosePolicy(e policyEvidence) policyChoice {
	if e.disk.Path == "" || e.disk.Path == projectconfig.DefaultPath && e.committed.path == projectconfig.CanonicalPath {
		return committedOrNone(e.committed)
	}
	chosen := e.disk
	if e.committed.path == chosen.Path {
		if projectconfig.Digest(e.committed.content) == chosen.Digest || e.holdsConvertedAt(chosen.Path) {
			return usedPolicy(e.committed.path, e.committed.content, codesignal.PolicyCauseSameAsHead)
		}
		return usedPolicy(chosen.Path, chosen.Content, codesignal.PolicyCauseDiffersFromHead)
	}
	return usedPolicy(chosen.Path, chosen.Content, codesignal.PolicyCauseNotCommitted)
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
